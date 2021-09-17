package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type LineInfo struct {
	seq    string
	lineNo int64
	line   string
}

type FindInfo struct {
	filename string
	lines    []LineInfo
}

type Post struct {
	Body         string        `json:"body"`
	ConnectColor string        `json:"connectColor"`
	ConnectInfo  []SubMessage2 `json:"connectInfo"`
}

type SubMessage1 struct {
	title       string
	description string
}

type SubMessage2 struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageUrl    string `json:"imageUrl"`
}

func main() {
	var q []string

	if len(os.Args) < 4 {
		fmt.Println("3개 이상의 실행인자가 필요합니다.")
		fmt.Println("ex. ./search https://wh.jandi.com/connect-api/webhook/11671944/697c32db40d69a585b3a00c84dedcf73 ERROR filepath")
		return
	}

	hookUrl := os.Args[1]
	word := os.Args[2]
	files := os.Args[3:]

	for {
		findInfos := []FindInfo{}
		for _, path := range files {
			findInfos = append(findInfos, FindWordInAllFiles(word, path)...)
		}

		if len(findInfos) == 0 {
			fmt.Println("일치하는 정보가 없습니다.")
			continue
		}

		for _, findInfo := range findInfos {
			fmt.Println(findInfo.filename)
			fmt.Println("--------------------------------")

			if len(findInfo.lines) == 0 {
				fmt.Println("일치하는 라인을 찾을수 없습니다.")
				continue
			}

			for _, lineInfo := range findInfo.lines {
				if contains(q, lineInfo.seq) {
					continue
				}

				sendMessage(hookUrl, lineInfo.line)
				if len(q) == 5 {
					q = q[1:]
				}
				q = append(q, lineInfo.seq)
			}
			fmt.Println("--------------------------------")
			fmt.Println()
		}
		time.Sleep(3 * time.Second)
	}
}

func sendMessage(hookUri string, message string) {
	// check hooking url
	postUrl, err := url.Parse(hookUri)

	if err != nil {
		fmt.Println("Malformed URL: ", err.Error())
		return
	}

	subMessage2 := &SubMessage2{
		Title:       message[:20],
		Description: message,
		ImageUrl:    "https://golang.org",
	}

	tmpList := []SubMessage2{}
	tmpList = append(tmpList, *subMessage2)
	tmp := Post{
		message[:20],
		"#FAC11B",
		tmpList,
	}

	paylaodBuf := new(bytes.Buffer)
	json.NewEncoder(paylaodBuf).Encode(tmp)

	request, error := http.NewRequest("POST", postUrl.String(), paylaodBuf)
	request.Header.Set("Accept", "application/vnd.tosslab.jandi-v2+json")
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, error := client.Do(request)
	if error != nil {
		panic(error)
	}
	defer response.Body.Close()

	fmt.Println("response Status:", response.Status)
	fmt.Println("response Headers:", response.Header)
	body, _ := ioutil.ReadAll(response.Body)
	fmt.Println("response Body:", string(body))
}

func FindWordInAllFiles(word, path string) []FindInfo {
	findInfos := []FindInfo{}
	filelist, err := filepath.Glob(path)
	if err != nil {
		fmt.Println("파일 경로가 잘못되었습니다. err:", err, "path:", path)
		return findInfos
	}

    if len(filelist) == 0 {
        return findInfos
    }

	ch := make(chan FindInfo)
	cnt := len(filelist)
	recvCnt := 0

	for _, filename := range filelist {
		go FindWordInFile(word, filename, ch)
	}

	for findInfo := range ch {
		findInfos = append(findInfos, findInfo)
		recvCnt++
		if recvCnt == cnt {
			break
		}
	}
	return findInfos
}

func FindWordInFile(word, filename string, ch chan FindInfo) {
	findInfo := FindInfo{filename, []LineInfo{}}
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("파일을 찾을 수 없습니다. ", filename)
		ch <- findInfo
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	filesize := stat.Size()
	print("filesize: ", filesize)
	print("\n")

	var lineNo int64 = 1
	scanner := bufio.NewScanner(file)
	r, _ := regexp.Compile("[0-9]{1,4}/[0-9]{1,2}/[0-9]{1,2} [0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}")
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, word) {
			result := r.FindString(line)
			findInfo.lines = append(findInfo.lines, LineInfo{result, lineNo, line})
		}
		lineNo++
	}

	infoLength := len(findInfo.lines)
    if len(findInfo.lines) > 5 {
	  findInfo.lines = findInfo.lines[infoLength-5:]
    }

	ch <- findInfo
}

func contains(s []string, substr string) bool {
	for _, v := range s {
		if v == substr {
			return true
		}
	}
	return false
}
