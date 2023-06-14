package util

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var _httpClient = &http.Client{
	Timeout: time.Duration(5) * time.Minute,
	Transport: &http.Transport{
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   5,
		MaxConnsPerHost:       5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

func DownloadRequest(filepath string, url string) bool {
	start := time.Now()

	length, success := ContentLengthRequest(url)
	if !success || length == 0 {
		log.Printf("ContentLength fail. url: `%s`. path: `%s`. length: `%d`. \n", url, filepath, length)
		return false
	}

	fi, err := os.Stat(filepath)
	if err == nil {
		if length == fi.Size() {
			return true
		}
		err = os.Remove(filepath)
		log.Printf("File remove. url: `%s`. path: `%s`. err: `%s`. \n", url, filepath, err)
	}

	out, err := os.Create(filepath)
	if err != nil {
		log.Printf("Create file error. url: `%s`. path: `%s`. e: `%s`. \n", url, filepath, err)
		return false
	}
	defer out.Close()

	resp, err := _httpClient.Get(url)
	if err != nil {
		log.Printf("Download fail. url: `%s`. err: `%s`. \n", url, err)
		return false
	}
	defer resp.Body.Close()

	l, e := io.Copy(out, resp.Body)
	log.Printf("Download success. url: `%s`. path: `%s`. cost: `%d`. l: `%d`. err: `%s`. \n", url, filepath, time.Since(start).Milliseconds(), l, e)

	fi, _ = os.Stat(filepath)
	return length == fi.Size()
}

func ContentLengthRequest(url string) (int64, bool) {
	start := time.Now()

	resp, err := _httpClient.Head(url)
	if err != nil {
		log.Printf("ContentLength fail. url: `%s`. err: `%s`. \n", url, err)
		return 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ContentLength status fail. url: `%s`. status: `%d`. \n", url, resp.StatusCode)
		return 0, false
	}

	_, err = io.ReadAll(resp.Body)

	log.Printf("ContentLength success. url: `%s`. cost: `%d`.\n", url, time.Since(start).Milliseconds())
	return resp.ContentLength, true
}

func GetRequest(url string, header map[string]string) (string, bool) {
	bs, success := httpGet(url, header)
	if !success {
		return "", false
	}
	return string(bs), true
}

func httpGet(url string, header map[string]string) ([]byte, bool) {
	start := time.Now()

	request, _ := http.NewRequest("GET", url, nil)
	for k, v := range header {
		request.Header.Set(k, v)
	}

	response, err := _httpClient.Do(request)
	if err != nil {
		log.Printf("Http get error. url: `%s`. err: `%s`. \n", url, err)
		return nil, false
	}
	defer response.Body.Close()

	httpCode := response.StatusCode
	if httpCode != http.StatusOK {
		log.Printf("Http get status error. url: `%s`. httpCode: `%d`. cost: %d. err: `%s`. \n", url, httpCode, time.Since(start).Milliseconds(), err)
		return nil, false
	}
	log.Printf("Http get success. url: `%s`. httpCode: `%d`. cost: %d. \n", url, httpCode, time.Since(start).Milliseconds())

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Body to string error. url: `%s`. err: `%s`. \n", url, err)
		return nil, false
	}
	return bytes, true
}

func CreateHeader() map[string]string {
	header := make(map[string]string)
	header["user-agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36"
	return header
}

func ParseDownloadFileName(url string) string {
	var fileName string
	if strings.Contains(url, "?") {
		fileName = url[strings.LastIndex(url, "/")+1 : strings.LastIndex(url, "?")]
	} else {
		fileName = url[strings.LastIndex(url, "/")+1:]
	}
	return fileName
}
