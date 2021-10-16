package util

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

//gzip.BestCompression
func GzipCompress(a *[]byte,level int) []byte {
	var b bytes.Buffer
	gz,err := gzip.NewWriterLevel(&b,level)
	if err != nil {
		panic(err)
	}
	if _, err := gz.Write(*a); err != nil {
		gz.Close()
		panic(err)
	}
	gz.Close()
	return b.Bytes()
}

func GzipDecompress(data *[]byte)([]byte,error)  {

	reader := bytes.NewReader(*data)
	gzreader, err := gzip.NewReader(reader)
	if err != nil{
		return nil,err
	}

	output, err := ioutil.ReadAll(gzreader)
	if err != nil{
		return nil,err
	}

	return output,nil
}
