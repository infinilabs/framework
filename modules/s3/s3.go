/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package s3

import (
	"context"
	"crypto/tls"
	log "github.com/cihub/seelog"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type S3Config struct {
	Endpoint string `config:"endpoint" json:"endpoint,omitempty"`
	AccessKey string `config:"access_key" json:"access_key,omitempty"`
	AccessSecret string `config:"access_secret" json:"access_secret,omitempty"`
	Token string `config:"token" json:"token,omitempty"`
	SSL bool `config:"ssl" json:"ssl,omitempty"`
	SkipInsecureVerify bool `config:"skip_insecure_verify" json:"skip_insecure_verify,omitempty"`
}

type S3Module struct {

	//LatestFile map[string]int64 `config:"latest" json:"latest,omitempty"`

	S3Configs map[string] S3Config
}

type S3Uploader struct {
	S3Config *S3Config
	minioClient *minio.Client
}

func NewS3Uploader(cfg *S3Config)(*S3Uploader,error)  {

	// Keep TLS config.
	tlsConfig := &tls.Config{}
	if cfg.SkipInsecureVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	var transport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
		// Set this value so that the underlying transport round-tripper
		// doesn't try to auto decode the body of objects with
		// content-encoding set to `gzip`.
		//
		// Refer:
		//    https://golang.org/src/net/http/transport.go?h=roundTrip#L1843
		DisableCompression: true,
	}

	var err error
	uploader:=&S3Uploader{S3Config: cfg}
	uploader.minioClient, err = minio.New(uploader.S3Config.Endpoint, &minio.Options{
		Transport:transport,
		Creds:  credentials.NewStaticV4(uploader.S3Config.AccessKey, uploader.S3Config.AccessSecret, uploader.S3Config.Token),
		Secure: uploader.S3Config.SSL,

	})
	if err != nil {
		return nil,err
	}
	return uploader,nil
}

func (uploader *S3Uploader) AsyncUpload(filePath,location,bucketName,objectName string) (error){
	//TODO to tracking tasks, control concurrent workers
	go uploader.SyncUpload(filePath,location,bucketName,objectName)
	return nil
}

func (uploader *S3Uploader) SyncUpload(filePath,location,bucketName,objectName string) (bool,error){

	log.Tracef("s3 uploading file:%v to: %v",filePath,objectName)

	log.Tracef("s3 server [%v] is online:%v\n", uploader.minioClient.EndpointURL(),uploader.minioClient.IsOnline())

	var err error
	ctx := context.Background()
	err = uploader.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		exists, errBucketExists := uploader.minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Tracef("we already own %s", bucketName)
		} else {
			return false,err
		}
	} else {
		log.Tracef("successfully created %s", bucketName)
	}

	contentType := "application/zip"

	info, err := uploader.minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Error(info,err)
		return false,err
	}

	log.Debugf("successfully uploaded %s of size %d", objectName, info.Size)

	return true, nil
}
var locker= sync.Mutex{}
func (uploader *S3Uploader) SyncDownload(filePath,location,bucketName,objectName string) (bool,error){

	log.Tracef("try downloading s3 file:%v to: %v",objectName,filePath)

	if !uploader.minioClient.IsOnline(){
		log.Tracef("s3 server [%v] is online:%v\n", uploader.minioClient.EndpointURL(),uploader.minioClient.IsOnline())
		return false,errors.New("s3 server is offline")
	}

	var err error
	ctx := context.Background()

	locker.Lock()
	defer locker.Unlock()

	exists, errBucketExists := uploader.minioClient.BucketExists(ctx, bucketName)
	if errBucketExists != nil || !exists {
		log.Tracef("bucket not exists %s, %v", bucketName,errBucketExists)
		return false,err
	}

	if util.FileExists(filePath){
		log.Tracef("local file exists, %v, %v",objectName,filePath)
		return true,nil
	}

	tempPath:=filePath+".s3_tmp"
	if util.FileExists(tempPath){
		log.Warnf("s3 temp file exists, delete: ",tempPath)
		util.FileDelete(tempPath)
	}

	log.Debugf("s3 downloading file:%v to: %v",objectName,filePath)

	err = uploader.minioClient.FGetObject(ctx, bucketName, objectName, tempPath, minio.GetObjectOptions{})
	if err != nil {
		if global.Env().IsDebug{
			log.Error(err)
		}
		return false,err
	}
	if !util.FileExists(tempPath){
		return false,errors.New("download failed")
	}

	err=os.Rename(tempPath,filePath)
	if err != nil {
		if global.Env().IsDebug{
			log.Error(err)
		}
		return false,err
	}

	log.Debugf("successfully downloaded %s", objectName)

	return true, nil
}

func (module *S3Module) Name() string {
	return "S3"
}

func (module *S3Module) Setup() {
	var err error
	module.S3Configs=map[string]S3Config{}
	ok,err:=env.ParseConfig("s3", &module.S3Configs)
	if ok&&err!=nil{
		panic(err)
	}
	if ok{
		for k,v:=range module.S3Configs{
			handler,err:=NewS3Uploader(&v)
			if err!=nil{
				log.Error(err)
				continue
			}
			s3.Register(k,handler)
		}
	}

}

func (module *S3Module) Start() error {
	return nil
}

func (module *S3Module) Stop() error {

	return nil
}
