package main
//本脚本用于操作Linux的工具tc，来模拟网络延迟，丢包等，以实现类似chaos monkey的功能。

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)


func Executor(opt bool ,iface, delay, loss string ){
	//命令执行器.判断参数并生成目标命令
	OptCmd := []string{"/usr/sbin/tc", "qdisc", "-", "dev", "ens160", "root", "netem"}
	OptCmd[4] = iface

	if opt == true {
		OptCmd[2] = "del"
		fmt.Println("当前操作为: 删除规则")
	}else {
		OptCmd[2] = "add"
		fmt.Println("当前操作为: 添加规则")
	}

	if delay != "0" {
		OptCmd = append(OptCmd, "delay", delay + "ms")
		fmt.Printf(">>>延迟设置为%vms\n", delay)
	}
	if loss != "0" {
		if strings.Contains(loss, "%"){
			OptCmd = append(OptCmd, "loss", loss)
			fmt.Printf(">>>丢包率设置为%v\n", loss)
		}else{
			OptCmd = append(OptCmd, "loss", loss + "%")
			fmt.Printf(">>>丢包率设置为%v%%\n", loss)
		}
	}

	stdout, stderr := exec.Command(OptCmd[0], OptCmd[1:]...).Output()
	if stderr != nil {
		fmt.Println(stderr)
		fmt.Println("命令执行出错.")
		os.Exit(-1)
	}
	if len(stdout) > 0 {
		fmt.Println("命令执行成功\n",stdout)
	}else {
		fmt.Println("命令执行成功")
	}
}


func IfaceNames() []string{
	//网卡名迭代器,返回所有网卡名的slice
	var IFnames []string
	InterFaces, err := net.Interfaces()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	for _, i := range InterFaces{
		sn := fmt.Sprintf("%v", i.Name)
		IFnames = append(IFnames, sn)
	}

	return IFnames
}



func main() {
	opt := flag.Bool("r", false, "删除规则,可选项")
	iface := flag.String("i", " ", "网络适配器名称,必须指定")
	delay := flag.String("d", "0", "网络发包的延迟时间,单位毫秒,可选项")
	loss := flag.String("l", "0", "网络发包的丢包率,单位为百分比,1-99,无需加百分号,可选项")
	flag.Parse()

	if *iface == " " {
		fmt.Println("必须指定网卡名")
		os.Exit(-1)
	}

	if *delay == "0" && *loss == "0" && *opt == false{
		fmt.Println("无有效故障注入参数,未执行任何操作,退出.")
		os.Exit(-1)}

	FoundInterface := false
	for _, ifn := range IfaceNames(){
		if ifn == *iface {
			FoundInterface = true
			Executor(*opt, *iface, *delay, *loss)
		}
	}
	if FoundInterface == false {
		fmt.Println("网卡名找不到,请检查.")
	}

}