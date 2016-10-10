package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"strconv"

	"code.google.com/p/mahonia"
)

var base = flag.String("f", "", "from svn 源地址")
var targetDir = flag.String("t", "export/", "to folder 导出目录")
var r = flag.String("r", "-1", "revision 版本")
var isPrint = flag.Bool("i", false, "print fileList or not 是否打印文件列表")
var password = flag.String("p", "", "password 密码")
var username = flag.String("u", "", "username 用户名")

var local bool           //will copy form local svn 是否操作本地svn,如果为否,则代表远程url操作服务端
var reg *regexp.Regexp   //remove "M" "A" "D" strings 修改.添加.删除
var regrn *regexp.Regexp //split to lines 用于分行

func main() {
	reg = regexp.MustCompile(`[MAD][ ]+`)
	regrn = regexp.MustCompile(`\n`)

	flag.Parse()
	if len(flag.Args()) != 0 {
		if flag.Arg(0) == "help" || flag.Arg(0) == "-help" || flag.Arg(0) == "-h" || flag.Arg(0) == "h" || flag.Arg(0) == "man" {
			flag.Usage()
		}
		return
	}

	if *r == "-1" {
		fmt.Println("请用 -r 3245 的形式指定起始版本号")
		return
	} else {
		fmt.Println("起始版本号:" + *r)
	}

	if *username == "" {
		local = true
	}
	targetDir = addEnd(targetDir)
	if *base != "" {
		base = addEnd(base)
	}

	newbase := strings.Replace(*base, "\\", "/", -1)
	base = &newbase

	var cmd *exec.Cmd

	//检查是否是最新版本, 不是则需要用户先进行 svn update
	cmd = exec.Command("svnversion", *base)
	out, err := cmd.CombinedOutput()
	check(err)
	ver := string(out)
	fmt.Println(ver)
	if local && strings.Index(ver, ":") >= 0 {
		fmt.Print("\n\n警告: 你还没有使用 svn update 更新本地,是否继续运行? (y继续,n退出) \n请输入你的选择:")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if strings.Index(text, "n") >= 0 {
			return
		}
	}

	infopath := "info.json"
	err = ioutil.WriteFile(infopath, []byte(ver), 0666)
	check(err)

	os.Mkdir(*targetDir, 0777)

	cmd = exec.Command("svn", "diff", *base, "--summarize", "-r", *r+":HEAD", "--username", *username, "--password", *password)
	if local {
		cmd = exec.Command("svn", "diff", *base, "--summarize", "-r", *r+":HEAD")
	}

	out, err = cmd.CombinedOutput()
	check(err, "错误: svn diff")
	str, err := url.QueryUnescape(string(out))
	check(err, "错误: url.QueryUnescape")
	if local {
		str = string(out)
		dec := mahonia.NewDecoder("gbk")
		str = dec.ConvertString(str)
	}

	tmps := regrn.Split(str, -1)

	count := len(tmps)
	sort.Sort(sort.Reverse(sort.StringSlice(tmps)))

	if *isPrint {
		fmt.Println("--------------")
	}
	for i := 0; i < count; i++ {
		filePath := strings.TrimSpace(tmps[i])
		tmps[i] = filePath
		if *isPrint {
			fmt.Println(filePath)
		}
	}

	for i := 0; i < count; i++ {
		filePath := tmps[i]
		if len(filePath) > 1 {
			if filePath[0] == 'M' || filePath[0] == 'A' {
				filePath = reg.ReplaceAllLiteralString(filePath, "") //去掉 M 开头的空白字符串
				fmt.Print(".")
				if local {
					filePath = strings.Replace(filePath, "\\", "/", -1)
				}
				pathRelative := strings.Replace(filePath, *base, "", -1)
				arr := strings.Split(pathRelative, "/")
				arr = arr[0:(len(arr) - 1)]
				dirNeedToMake := *targetDir + strings.Join(arr, "/")
				err = os.MkdirAll(dirNeedToMake, 0777)
				check(err, "在 MkdirAll 之后")
				pathRelative = strings.TrimSpace(pathRelative)
				outputPath := *targetDir + pathRelative
				if local {
					_, err = copyFile(filePath, outputPath)
					//check(err, "在 copyFile 之后")
				} else {
					cmd = exec.Command("svn", "export", "--depth=infinity", "--force", "-q", "-r", "HEAD", filePath, outputPath, "--username", *username, "--password", *password)
					out, err = cmd.CombinedOutput()
					if err != nil {
						fmt.Println(string(out))
					}
					check(err, "在 调用 svn export 之后的报错")
				}
			} else {
				//fmt.Println("删除 " + filePath)
			}
		}
	}
	fmt.Println("\n" + strconv.Itoa(count) + "行数据")
	fmt.Println("done.\n ")
}

func addEnd(str *string) *string {
	count := len(*str)
	end := Substr(*str, count-1, 1)
	if end != "/" {
		newstr := *str + "/"
		return &newstr
	}
	return str
}

func check(err error, desc ...string) {
	if err != nil {
		fmt.Printf(strings.Join(desc, " ") + ": ")
		fmt.Println(err)
	}
}

//复制文件
func copyFile(src, des string) (w int64, err error) {
	srcFile, err := os.Open(src)
	check(err, "源文件错误(可能需要 svn update)")
	desFile, err := os.Create(des)
	if err != nil {
		errstr := err.Error()
		index := strings.Index(errstr, "directory")
		if index <= 0 {
			fmt.Printf("目标文件错误: ")
			fmt.Println(errstr)
		}
	}

	defer srcFile.Close()
	defer desFile.Close()
	return io.Copy(desFile, srcFile)
}

//截取字符串 start 起点下标 length 需要截取的长度
func Substr(str string, start int, length int) string {
	rs := []rune(str)
	rl := len(rs)
	end := 0

	if start < 0 {
		start = rl - 1 + start
	}
	end = start + length

	if start > end {
		start, end = end, start
	}

	if start < 0 {
		start = 0
	}
	if start > rl {
		start = rl
	}
	if end < 0 {
		end = 0
	}
	if end > rl {
		end = rl
	}

	return string(rs[start:end])
}

//截取字符串 start 起点下标 end 终点下标(不包括)
func Substr2(str string, start int, end int) string {
	rs := []rune(str)
	length := len(rs)

	if start < 0 || start > length {
		panic("start is wrong")
	}

	if end < 0 || end > length {
		panic("end is wrong")
	}

	return string(rs[start:end])
}
