package main
//本脚本用于扫描指定目录下的.log文件，并将文件分别打包压缩。可指定选项删除源文件。
import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)



func scanner(scan_path string) []string {
	//路径扫描,返回所有对象
	var file_list []string
	scan_folder := filepath.Join(scan_path + "/*")
	abs_path, err := filepath.Abs(scan_path)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("文件搜索路径为:", abs_path)
	file_temp_list, err := filepath.Glob(scan_folder)
	if err != nil {
		fmt.Println("出现报错:", err)
	}
	fmt.Printf("文件/目录对象共有%d个\n\n", len(file_temp_list))

	for _, file := range file_temp_list {
		file_path, err := filepath.Abs(file)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		file_list = append(file_list, file_path)
	}

	return file_list
}


func filter(file_list []string)  []string{
	//过滤掉软链接和目录
	//file_for_compress := []string
	var folders  []string
	var files  []string
	for _, file := range file_list {
		file_info, err := os.Lstat(file)
		if err == nil {
			if file_info.IsDir() {
				//fmt.Printf("%s 是目录\n", file_info.Name())
				folders = append(folders, file_info.Name())
			}else if  file_info.Mode() & os.ModeSymlink == os.ModeSymlink{
				fmt.Printf("%s 是软链接\n", file_info.Name())
			}else{
				//fmt.Printf("%s 是文件\n", file_info.Name())
				files = append(files, file_info.Name())
			}
		}else {
			fmt.Printf("%v\n", err)
		}
	}

	return files
}


func zip_files(src_file string) error {
	//文件压缩
	//参考 https://golangcode.com/create-zip-files-in-go/
	filename := filepath.Base(src_file)
	file_base_name := filename[:len(filename) - len(filepath.Ext(filename))]
	zip_file_name := file_base_name + ".zip"

	new_zipfile, err := os.Create(zip_file_name)
	if err != nil {
		return err
	}
	defer new_zipfile.Close()

	zip_writer := zip.NewWriter(new_zipfile)
	defer zip_writer.Close()

	file_for_zip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file_for_zip.Close()

	info, err := file_for_zip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filename
	header.Method = zip.Deflate

	writer, err := zip_writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err = io.Copy(writer, file_for_zip); err != nil {
		return err
	}

	return nil
}


func remove_file(file_name string)  error{
	//原文件删除
	err := os.Remove(file_name)
	if err != nil {
		return err
	}
	fmt.Printf("   文件 %s 已删除\n", file_name)
	return nil
}



func main() {

	log_path := flag.String("p", "./", "被压缩日志所在的目录名")
	remove := flag.Bool("d", false, "是否删除原文件,默认为false")
	flag.Parse()

	real_log_path, err := filepath.Abs(*log_path)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("========本工具只压缩指定目录下的.log文件========")
	os.Chdir(real_log_path)
	files_for_zip := filter(scanner(real_log_path))

	var files_for_zip_real []string
	for _, src_file := range files_for_zip {
		if filepath.Ext(src_file) != ".log" {
			continue
		}
		files_for_zip_real = append(files_for_zip_real, src_file)
	}
	fmt.Printf("\n================\n待压缩文件为:\n")
	for _, file := range files_for_zip_real {
		fmt.Println(file)
	}
	fmt.Println("================", "\n")


	for _, src_file := range files_for_zip {
		if filepath.Ext(src_file) != ".log" {
			fmt.Printf("忽略文件 %s \n", src_file)
			continue
		}
		fmt.Printf(">>>开始压缩文件 %s ...\n", src_file)
		err := zip_files(src_file)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("   %s 压缩完成.\n", src_file)

		if *remove == true {
			remove_file(src_file)
		}
	}
	fmt.Println("\n========操作全部完成.========")
}