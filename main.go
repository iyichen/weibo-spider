package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"html/template"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"weibo-spider/util"
)

var pwd, _ = os.Getwd()
var wg sync.WaitGroup
var requestHeader = util.CreateHeader()
var tmpl, _ = template.ParseFiles(pwd + "/template.html")

func main() {
	runtime.GOMAXPROCS(10)

	for {
		util.Println("准备开始获取数据...")
		fetch()
		util.Println("获取数据完成，30分后进行下一次获取...")
		time.Sleep(30 * time.Minute)
	}

	//util.StopTerminalDisappear()
}

func fetch() {

	targets := util.Read(pwd + "/weibo.txt")
	if targets == "" {
		util.Println("待抓取地址不存在")
		return
	}

	cookie := util.Read(pwd + "/cookie.txt")
	if cookie == "" {
		util.Println("cookie不存在")
		return
	}
	requestHeader["cookie"] = cookie

	var dirs []string
	for _, homeUrl := range strings.Split(targets, "\n") {
		homeUrl = strings.TrimSpace(homeUrl)
		if homeUrl == "" {
			continue
		}
		util.Println("准备开始获取数据：" + homeUrl)

		user := getUser(homeUrl)
		if user == nil {
			util.Println("获取用户信息失败")
			continue
		}

		dir := pwd + "/weibo/" + user.Id + "_" + util.Escape(user.Name)
		util.MakeDir(pwd + "/weibo/")

		ds, _ := os.ReadDir(pwd + "/weibo/")
		for _, d := range ds {
			if !d.IsDir() {
				continue
			}
			if strings.Contains(d.Name(), "_") {
				id := d.Name()[0:strings.Index(d.Name(), "_")]
				if id == user.Id {
					//dir = pwd + "/weibo/" + d.Name()
					_ = os.Rename(pwd+"/weibo/"+d.Name(), dir)
				}
			} else {
				if d.Name() == user.Id {
					_ = os.Rename(pwd+"/weibo/"+d.Name(), dir)
				}
			}
		}
		util.MakeDir(dir)
		util.MakeDir(dir + "/images/")
		util.MakeDir(dir + "/data/")
		dirs = append(dirs, dir)

		util.Println("开始获取历史全量数据...")
		queryData(user.Id, dir, false)
		util.Println("获取历史全量数据完成...")
		util.Println("开始获取当前增量数据...")
		queryData(user.Id, dir, true)
		util.Println("获取当前增量数据完成...")

		time.Sleep(time.Second * 5)
	}

	// 开始下载图片
	util.Println("准备处理图片和页面...")
	for _, dir := range dirs {
		dataFiles, _ := os.ReadDir(dir + "/data")
		util.Println("开始处理图片和页面: " + dir)
		for _, file := range dataFiles {
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			filePath := dir + "/data/" + file.Name()
			util.Println("开始下载图片.数据文件：" + file.Name())
			weibos := loadWeiboData(filePath)
			for _, weibo := range weibos {
				if downloadPic(weibo) {
					time.Sleep(time.Millisecond * 400)
				}
			}
			util.Println("完成下载图片.数据文件：" + file.Name())
			saveWeiboData(filePath, weibos)
			renderHtml(weibos, dir)
			util.Println("完成页面创建.数据文件：" + file.Name())
			time.Sleep(time.Millisecond * 300)
		}
	}
}

func queryData(uid string, dir string, head bool) {
	sinceId := ""
	firstIds, lastId := getBreakpoint(dir)
	if !head && lastId > 0 {
		sinceId = strconv.FormatInt(lastId, 10)
	}

	weibos := make([]*Weibo, 0)
	weiboIds := make(map[int64]bool)
loop:
	for {
		blogUlr := "https://weibo.com/ajax/statuses/mymblog?uid=" + uid + "&feature=0"
		if sinceId != "" {
			blogUlr = blogUlr + "&since_id=" + sinceId
		}
		blogData, success := util.GetRequest(blogUlr, requestHeader)
		if !success {
			util.Println("网络请求失败，请检查网络设置")
			break
		}
		if !gjson.Valid(blogData) {
			util.Println("无法获取数据，请检查cookie设置")
			break
		}
		dataJson := gjson.Parse(blogData).Get("data")
		if !dataJson.Exists() || len(dataJson.Get("list").Array()) <= 1 {
			break // last page
		}

		for _, weiboJson := range dataJson.Get("list").Array() {
			weibo := parseWeibo(weiboJson, dir)
			retweet := weiboJson.Get("retweeted_status")
			if retweet.Exists() {
				weibo.Retweet = parseWeibo(retweet, dir)
			}

			if weiboIds[weibo.Id] {
				continue
			}
			weiboIds[weibo.Id] = true

			if head && firstIds[weibo.Id] { // 如果从头开始 && 查询到已经存在的数据，认为是新数据获取结束
				break loop
			}

			if len(weibos) == 0 || weibos[len(weibos)-1].Month == weibo.Month {
				// 相同月份，继续查找
				weibos = append(weibos, weibo)
			} else {
				// 不同月份，写入数据
				saveWeiboJsonData(dir, weibos)
				util.Println("写入" + weibos[len(weibos)-1].Month + "数据完成")
				weibos = []*Weibo{weibo}
			}

			sinceId = strconv.FormatInt(weibo.Id, 10)
		}
		time.Sleep(time.Millisecond * 600)
	}
	if len(weibos) > 0 {
		saveWeiboJsonData(dir, weibos)
		util.Println("写入" + weibos[len(weibos)-1].Month + "数据完成")
	}
}

func saveWeiboJsonData(dir string, weibos []*Weibo) {
	dataPath := dir + "/data/" + weibos[len(weibos)-1].Month + ".json"
	data := loadWeiboData(dataPath)
	dataMap := make(map[int64]bool)
	for _, e := range data {
		dataMap[e.Id] = true
	}
	flag := false
	for _, e := range weibos {
		if !dataMap[e.Id] {
			flag = true
			data = append(data, e)
		}
	}

	if flag {
		saveWeiboData(dataPath, data)
	}
}

func parseWeibo(weiboJson gjson.Result, dir string) *Weibo {
	weibo := Weibo{}
	weibo.Id = weiboJson.Get("id").Int()
	weibo.Name = weiboJson.Get("user.screen_name").Str
	t, _ := time.Parse(time.RubyDate, weiboJson.Get("created_at").Str)
	weibo.Time = t.Format(time.DateTime)
	weibo.Month = t.Format("2006-01")
	weibo.Content = weiboJson.Get("text_raw").Str
	if strings.Contains(weiboJson.Get("text").Str, "展开") {
		textId := weiboJson.Get("mblogid").Str
		textData, success := util.GetRequest("https://weibo.com/ajax/statuses/longtext?id="+textId, requestHeader)
		if success {
			text := gjson.Get(textData, "data.longTextContent").Str
			if text != "" {
				weibo.Content = text
			}
		}
	}
	for _, image := range weiboJson.Get("pic_ids").Array() {
		url := weiboJson.Get("pic_infos." + image.Str + ".largest.url").Str
		if url == "" {
			continue
		}
		weibo.Images = append(weibo.Images, &Image{
			Remote: url,
			Local:  dir + "/images/" + util.ParseDownloadFileName(url),
			Html:   "images/" + util.ParseDownloadFileName(url),
			Sync:   false,
		})
	}
	return &weibo
}

// 获取用户信息
func getUser(homeUrl string) *WeiboUser {

	if strings.Contains(homeUrl, "?") {
		homeUrl = homeUrl[0:strings.Index(homeUrl, "?")]
	}

	// https://weibo.com/u/3783359617
	if strings.Contains(homeUrl, "/u/") {
		id := homeUrl[strings.Index(homeUrl, "/u/")+3:]
		infoData, success := util.GetRequest("https://weibo.com/ajax/profile/info?uid="+id, requestHeader)
		if !success {
			util.Println("网络请求失败，请检查网络设置")
			return nil
		}
		userJson := gjson.Parse(infoData).Get("data").Get("user")
		if userJson.Exists() {
			user := &WeiboUser{
				Id:   userJson.Get("idstr").Str,
				Name: userJson.Get("screen_name").Str,
			}
			return user
		}
		return nil
	}

	// https://weibo.com/archifind
	name := homeUrl[strings.LastIndex(homeUrl, "/")+1:]

	infoData, success := util.GetRequest("https://weibo.com/ajax/profile/info?custom="+name, requestHeader)
	if !success {
		util.Println("网络请求失败，请检查网络设置")
		return nil
	}
	userJson := gjson.Parse(infoData).Get("data").Get("user")
	if userJson.Exists() {
		user := &WeiboUser{
			Id:   userJson.Get("idstr").Str,
			Name: userJson.Get("screen_name").Str,
		}
		return user
	}
	return nil
}

func loadWeiboData(file string) []*Weibo {
	var data []*Weibo
	content := util.Read(file)
	if content != "" {
		_ = json.Unmarshal([]byte(content), &data)
	}
	return data
}

func saveWeiboData(file string, weibos []*Weibo) {
	bs, _ := json.Marshal(weibos)
	util.Write(file, string(bs))
}

func getBreakpoint(dir string) (map[int64]bool, int64) {
	var lastId int64
	var firstIds = make(map[int64]bool)
	files, _ := os.ReadDir(dir + "/data")
	lastFileName := ""
	firstFileName := ""
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		if lastFileName == "" {
			lastFileName = file.Name()
			firstFileName = file.Name()
		} else if file.Name() < lastFileName {
			lastFileName = file.Name()
		} else if file.Name() > firstFileName {
			firstFileName = file.Name()
		}
	}
	if lastFileName != "" {
		data := loadWeiboData(dir + "/data/" + lastFileName)
		if len(data) != 0 {
			lastId = data[len(data)-1].Id
		}
	}
	if firstFileName != "" {
		data := loadWeiboData(dir + "/data/" + firstFileName)
		for _, e := range data {
			firstIds[e.Id] = true
		}
	}
	return firstIds, lastId
}

func downloadPic(weibo *Weibo) bool {
	var images []*Image

	if len(weibo.Images) > 0 {
		images = append(images, weibo.Images...)
	}
	if weibo.Retweet != nil && len(weibo.Retweet.Images) > 0 {
		images = append(images, weibo.Retweet.Images...)
	}

	flag := false
	for i := 0; i < len(images); i++ {
		image := images[i]
		if image.Sync {
			continue
		}
		flag = true
		wg.Add(1)
		go func() {
			defer wg.Done()

			success := util.DownloadRequest(image.Local, image.Remote)
			if !success {
				util.Println(fmt.Sprintf("文件下载失败. 地址：%s.", image.Remote))
			}
			image.Sync = success
		}()
	}
	wg.Wait()
	return flag
}

func renderHtml(weibos []*Weibo, dir string) {
	month := weibos[0].Month
	var bs bytes.Buffer
	_ = tmpl.Execute(&bs, weibos)
	_ = os.WriteFile(dir+"/"+month+".html", bs.Bytes(), 0777)
}

type Weibo struct {
	Id      int64    `json:"id"`
	Name    string   `json:"name"`
	Time    string   `json:"time"`
	Month   string   `json:"month"`
	Content string   `json:"content"`
	Images  []*Image `json:"images"`
	Retweet *Weibo   `json:"retweet"`
}

type Image struct {
	Remote string `json:"remote"`
	Local  string `json:"local"`
	Html   string `json:"html"`
	Sync   bool   `json:"sync"`
}

type WeiboUser struct {
	Id   string
	Name string
}
