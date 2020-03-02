package main
//本程序用于从CoreDNS后端存储Etcd中取出所有DNS记录，并根据输入关键字做模糊搜索，并显示相关性最高的5条，按相关性排序
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fagongzi/util/uuid"
	"github.com/sahilm/fuzzy"
	"github.com/satori/go.uuid"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/testutil"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)


type  rrStruct struct {
	//etcd中的DNS记录值
	Host string `json:"host"`
	TTL int `json:"ttl"`
	ID string `json:"id"`
}

type Records struct {
	//CNAME和A记录列表,参与转换用
	CNFull []string
	CNnoID []string
	CNDomain []string
	CNHost []string

	AFull []string
	AnoID []string
}

//CNAME和A记录全列表
var allRecords Records

func checks(e error){
	//错误处理
	if 	e != nil {
		panic(e)
	}
}

func RangePrinter(ss []string){
	for _, s := range ss{
		fmt.Println(s)
	}
}

func MkUUID() string{
	//生成UUID
	if id, err := uuid.NewV4(); err != nil{
		errinfo := fmt.Sprintf("UUID creating error: %v\n" , err)
		panic(errinfo)
	}else {
		return id.String()}
}

func RRMaker(rr ,ip, uuid string) (string, string){
	//生成输入etcd的key,value
	rr_key := fmt.Sprintf("/coredns%s", rr)
	rr_value := fmt.Sprintf(`'{"host":"%s","ttl":36, "id":"%s"}'`,ip, uuid)
	return rr_key, rr_value

}

func RRNames() []string{
	//从文本文件读取records
	var words []string
	fd, err := ioutil.ReadFile("prod_records.txt")
	if err != nil{
		panic(err)
	}
	src_string := string(fd)
	words = strings.Split(src_string, "\n")
	//fmt.Println(words)
	return words
}

func Connector() *clientv3.Client{
	conn, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"http://etcd-0.etcd:2379", "http://etcd-1.etcd:2379", "http://etcd-2.etcd:2379"},
		DialTimeout: 3 * time.Second,
	})
	checks(err)
	return conn
}

func GetAllRecords(conn *clientv3.Client) {
	//查询并整理所有资源记录
	type RR struct {
		Key string
		Value rrStruct
	}
	var RRs  []RR

	ctx, cancel := context.WithTimeout(context.Background(), testutil.RequestTimeout)
	resp, _ := conn.Get(ctx, "/coredns/", clientv3.WithPrefix())

	//迭代所有查到的记录,存放到RR结构体
	for _, KVS := range resp.Kvs{
		var rr RR
		var value rrStruct
		err := json.Unmarshal([]byte(KVS.Value), &value)
		checks(err)
		rr.Value = value
		key := strings.Split(fmt.Sprintf(string(KVS.Key)), "/")
		tmpstr := []string{}
		for _, s := range key{
			if len(s) != 0 && s != "coredns"{
				tmpstr = append([]string{s}, tmpstr...)
			}
		}
		if JudgeRRType(value.Host) {
			tmpstr = tmpstr[1:]
		}
		final_key := strings.Join(tmpstr, ".")
		rr.Key = final_key
		RRs = append(RRs, rr)
	}
	cancel()

	//把上面的记录(A/CNAME)存放到allRecords.格式: "domainname host recordID"
	for _, R := range RRs {
		if JudgeRRType(R.Value.Host) {
			finalString := R.Key + " " + R.Value.Host + " " + R.Value.ID
			allRecords.AFull= append(allRecords.AFull, finalString)
		}else {finalString := R.Key + " " + R.Value.Host + " " + R.Value.ID
			allRecords.CNFull = append(allRecords.CNFull, finalString)
		}
	}

	//整理记录,按allRecord的结构要求
	for _, r := range allRecords.CNFull{
		tempStr := strings.Fields(r)
		dm, host, _:= tempStr[0], tempStr[1], tempStr[2]
		allRecords.CNnoID = append(allRecords.CNnoID, dm + " " + host)
		allRecords.CNDomain = append(allRecords.CNDomain, dm)
		allRecords.CNHost = append(allRecords.CNHost, host)
	}
	for _, r := range allRecords.AFull{
		tempStr := strings.Fields(r)
		dm, host, _ := tempStr[0], tempStr[1], tempStr[2]
		allRecords.AnoID = append(allRecords.AnoID, dm + " " + host)
	}
}

func StringsMatch(SrcStrings []string, strForMatch string) string{
	for _, str := range SrcStrings{
		if strings.Contains(str, strForMatch){
			return str

		}
	}
	return ""
}

func MatchStrOutput(matches fuzzy.Matches, counter int) []string {
	const bold = "\033[1m%s\033[0m"
	var matchedStrings []string
	for _, match := range matches {
		if counter > 0 {
			strLine := ""
			for i := 0; i < len(match.Str); i++ {
				if contains(i, match.MatchedIndexes) {
					strLine +=  fmt.Sprintf(bold, string(match.Str[i]))

				} else {
					strLine += string(match.Str[i])
				}

			}
			matchedStrings = append(matchedStrings, strLine)
			counter -= 1
		}
	}
	return matchedStrings
}

func FuzzyRR(keyword, domain_name, valuekw, typekw string, counter int) []string{
	if typekw == ""{
		typekw = "A"
	}
	typekw = strings.ToUpper(typekw)
	if !(typekw == "CNAME" || typekw == "A") {
		fmt.Println("类型输入错误.请输入CNAME/A")
		os.Exit(-1)
	}

	if typekw == "A"{
		finalKW := keyword + domain_name + valuekw
		return MatchStrOutput(fuzzy.Find(finalKW, allRecords.AnoID), counter)
	}else if typekw == "CNAME"{
		if valuekw == ""{
			finalKW := keyword + domain_name
			return MatchStrOutput(fuzzy.Find(finalKW, allRecords.CNnoID), counter)
		}else {
			prefixKW := keyword + domain_name
			sufixKW := valuekw
			var finalResults []string
			var commonResults [][]string
			prefixSearch := fuzzy.Find(prefixKW, allRecords.CNnoID)
			sufixSearch := fuzzy.Find(sufixKW, allRecords.CNHost)
			for _, pre := range prefixSearch{
				for _, suf := range sufixSearch{
					if strings.Contains(pre.Str, suf.Str){
						commonResults = append(commonResults, strings.Fields(pre.Str))
					}
				}
			}

			if len(commonResults) == 0 {
				fmt.Println("无符合条件记录")
				os.Exit(0)
			}
			for _, res := range commonResults{
				if counter > 0 {
					p := MatchStrOutput(fuzzy.Find(prefixKW, []string{res[0]}), 1)[0]
					s := MatchStrOutput(fuzzy.Find(sufixKW, []string{res[1]}), 1)[0]
					finalResults = append(finalResults, p + " " + s)
					counter -= 1
				}
			}
			//os.Exit(0)
			return finalResults
		}
	}
	return []string{}
}

func contains(needle int, haystack []int) bool {
	//测字符串包含
	for _, i := range haystack {
		if needle == i {
			return true
		}
	}
	return false
}

func JudgeRRType(RR string) bool {
	//根据是否是IP地址判断是否A记录
	res := net.ParseIP(RR)
	if res == nil {
		return false
	}else {	return true	}
}



//func main(){
//
//	//完整读取etcd所有项+keyword模糊搜索测试
//	start := time.Now().Nanosecond()
//
//	conn := Connector()
//	GetAllRecords(conn)
//	//res := FuzzyRR( "smtp", "example.net", "mail", "cname", 5)
//	res := FuzzyRR( "", "smtp.example.net", "", "A", 5)
//	for _, r := range res{
//		fmt.Println(r)}
//
//	fmt.Printf("搜索耗时: %v 毫秒\n", (time.Now().Nanosecond() - start )/1e6)
//
//}