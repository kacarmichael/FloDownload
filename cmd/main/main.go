package main

import (
	"bufio"
	"flag"
	"fmt"
	"m3u8-downloader/cmd/downloader"
	"m3u8-downloader/cmd/processor"
	"m3u8-downloader/cmd/transfer"
	"os"
	"strings"
)

func main() {
	url := flag.String("url", "", "M3U8 playlist URL")
	eventName := flag.String("event", "", "Event name")
	debug := flag.Bool("debug", false, "Enable debug mode")
	transferOnly := flag.Bool("transfer", false, "Transfer-only mode: transfer existing files without downloading")
	processOnly := flag.Bool("process", false, "Process-only mode: process existing files without downloading")

	flag.Parse()

	if *transferOnly {
		transfer.RunTransferOnly(*eventName)
		return
	}

	if *processOnly {
		processor.Process(*eventName)
		return
	}

	if *url == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter M3U8 playlist URL: ")
		inputUrl, _ := reader.ReadString('\n')
		inputUrl = strings.TrimSpace(inputUrl)
		downloader.Download(inputUrl, *eventName, *debug)
		return
	}

	downloader.Download(*url, *eventName, *debug)
}
