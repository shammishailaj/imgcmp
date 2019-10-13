package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

func main() {
	imageList := makeImageList(".")
	pullRequestMessage := []string{
		"[imgcmp] Optimize images",
		"",
		"## Successfully optimized",
	}
	reportTable := []string{
		"<details>",
		"",
		"<summary>details</summary>",
		"",
		"|File Name|Before|After|Diff (size)|Diff (rate)|",
		"|:---|---:|---:|---:|---:|",
	}
	var totalBeforeSize int64 = 0
	var totalAfterSize int64 = 0
	wg := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	// optimize images and make reportTable
	for _, path := range imageList {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			beforeSize := fileSize(p)
			optimizeImage(p)
			afterSize := fileSize(p)
			mutex.Lock()
			reportTable = append(reportTable, tableRow(p, beforeSize, afterSize))
			mutex.Unlock()
			totalBeforeSize += beforeSize
			totalAfterSize += afterSize
		}(path)
	}
	wg.Wait()

	// pull request
	if (totalAfterSize - totalBeforeSize) == 0 {
		fmt.Println("images are already optimized")
	} else {
		reportTable = append(reportTable, tableRow("Total", totalBeforeSize, totalAfterSize), "", "</details>")
		pullRequestMessage = append(
			pullRequestMessage,
			fmt.Sprintf(
				"Your image file size has been reduced by **%v** (**%v**)",
				diffRate(totalBeforeSize, totalAfterSize),
				byte2unit(totalAfterSize-totalBeforeSize),
			),
			"",
		)
		execCommand("git", []string{"add", "."})
		execCommand("git", []string{"commit", "-m", "Optimize images of " + os.Getenv("GITHUB_SHA")})
		execCommand("git", []string{"push", "origin", os.Getenv("REMOTE_BRANCH")})
		pullRequestMessage = append(pullRequestMessage, reportTable...)
		execCommand("hub", []string{"pull-request", "-m", strings.Join(pullRequestMessage, "\n")})
		fmt.Println("Successfully optimized")
	}
}

func makeImageList(root string) []string {
	skipDirRegexp := regexp.MustCompile(`^\..+`)
	imageList := []string{}
	callback := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if skipDirRegexp.MatchString(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if isJPEG(rel) || isPNG(rel) {
			imageList = append(imageList, rel)
		}
		return nil
	}
	err := filepath.Walk(root, callback)
	if err != nil {
		fmt.Println(1, err)
	}
	return imageList
}

func isJPEG(path string) bool {
	return regexp.MustCompile(`(\.[jJ][pP][eE]?[gG]$|\.[jJ][pP][eE]$)`).MatchString(path)
}

func isPNG(path string) bool {
	return regexp.MustCompile(`\.[pP][nN][gG]$`).MatchString(path)
}

func optimizeImage(path string) {
	if isJPEG(path) {
		execCommand("jpegoptim", []string{"-m85", path})
	} else if isPNG(path) {
		execCommand("optipng", []string{"-o2", path})
	}
}

func execCommand(command string, args []string) {
	_, err := exec.Command(command, args...).Output()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func fileSize(path string) int64 {
	file, errOpen := os.Open(path)
	if errOpen != nil {
		fmt.Println(errOpen.Error())
	}
	info, errStat := file.Stat()
	if errStat != nil {
		fmt.Println(errStat.Error())
	}
	return info.Size()
}

func byte2unit(size int64) string {
	var res string
	negative := size < 0
	size = int64(math.Abs(float64(size)))
	if size >= 1e9 {
		res = fmt.Sprintf("%.2f GB", float64(size)/1e9)
	} else if size >= 1e6 {
		res = fmt.Sprintf("%.2f MB", float64(size)/1e6)
	} else if size >= 1e3 {
		res = fmt.Sprintf("%.2f kB", float64(size)/1e3)
	} else {
		res = fmt.Sprintf("%v Byte", size)
	}
	if negative {
		res = "-" + res
	}
	return res
}

func diffRate(before int64, after int64) string {
	return fmt.Sprintf("%.2f", float64(after-before)/float64(before)*100) + "%"
}

func tableRow(name string, before int64, after int64) string {
	return fmt.Sprintf("|%v|%v|%v|%v|%v|", name, byte2unit(before), byte2unit(after), byte2unit(after-before), diffRate(before, after))
}