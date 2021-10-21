package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// 	"regexp"

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

	if len(os.Args) < 3 {
		fmt.Println("2개 이상의 실행인자가 필요합니다.")
		fmt.Println("ex. ./search https://wh.jandi.com/connect-api/webhook/11671944/697c32db40d69a585b3a00c84dedcf73 filepath")
		return
	}

	hookUrl := os.Args[1]
	files := os.Args[2:]

	rand.Seed(time.Now().UnixNano())
	c := time.Tick(3 * time.Second)
	for _ = range c {
		findInfos := []FindInfo{}
		for _, path := range files {
			findInfos = append(findInfos, GetLinesInAllFiles(path)...)
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

				fmt.Printf("hookUrl: %s\n", hookUrl)
				fmt.Printf("message: %s", lineInfo.line)

				sendMessage(hookUrl, lineInfo.line)
				if len(q) == 200 {
					q = q[1:]
				}
				q = append(q, lineInfo.seq)
			}
			fmt.Println("--------------------------------")
			fmt.Println()
		}
	}
}

func sendMessage(hookUri string, message string) {
	// check hooking url
	postUrl, err := url.Parse(hookUri)

	if err != nil {
		fmt.Println("Malformed URL: ", err.Error())
		return
	}

	msg := message
	if len(msg) > 500 {
		msg = msg[:500]
	}

	title := ""
	if len(msg) > 100 {
		title = msg[:100]
	}

	subMessage2 := &SubMessage2{
		Title:       title,
		Description: msg,
		ImageUrl:    "https://golang.org",
	}

	tmpList := []SubMessage2{}
	tmpList = append(tmpList, *subMessage2)
	tmp := Post{
		title,
		"#FAC11B",
		tmpList,
	}

	paylaodBuf := new(bytes.Buffer)
	json.NewEncoder(paylaodBuf).Encode(tmp)

	request, error := http.NewRequest("POST", postUrl.String(), paylaodBuf)
	check(error)

	request.Header.Set("Accept", "application/vnd.tosslab.jandi-v2+json")
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, error := client.Do(request)
	check(error)

	defer response.Body.Close()

	fmt.Println("response Status:", response.Status)
	fmt.Println("response Headers:", response.Header)
	body, _ := ioutil.ReadAll(response.Body)
	fmt.Println("response Body:", string(body))
}

func GetLinesInAllFiles(path string) []FindInfo {
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
		go GetLinesOfFile(filename, ch, 1000)
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

func GetLinesOfFile(filename string, ch chan FindInfo, lineNumber int64) {
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
	if filesize < 0 {
		fmt.Println("파일 사이즈가 너무 작습니다. ", filesize)
		ch <- findInfo
		return
	}

	fileLineNumber, err := lineCounter(file)
	check(err)

	if lineNumber > int64(fileLineNumber) {
		lineNumber = int64(fileLineNumber) - 1
	}

	var lineNo int64 = 1
	var cursor int64 = 0
	r, _ := regexp.Compile("[0-9]{1,4}/[0-9]{1,2}/[0-9]{1,2} [0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}")
	line := ""

	for {
		if lineNo > lineNumber {
			break
		}

		for {
			cursor -= 1
			file.Seek(cursor, io.SeekEnd)

			char := make([]byte, 1)
			_, err := file.Read(char)
			check(err)

			if cursor != -1 && (char[0] == 10 || char[0] == 13) {
				break
			}
			line = fmt.Sprintf("%s%s", string(char), line)

			if cursor == -filesize {
				break
			}
		}
		seq := ""
		seq = r.FindString(line)

		// hashValue := hash(line)
		// seq = strconv.FormatUint(uint64(hashValue), 10)

		if seq != "" {
			// put read data into findInfo
			findInfo.lines = append(findInfo.lines, LineInfo{seq, lineNo, line})
		}
		line = ""
		lineNo++
	}

	fmt.Println("+++++++++++++++++++++++++++")
	fmt.Println(findInfo.lines)
	fmt.Println(len(findInfo.lines))
	fmt.Println("+++++++++++++++++++++++++++")

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

func check(e error) {
	if e != nil && e != io.EOF {
		// panic(e)
		fmt.Println(e)
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}
