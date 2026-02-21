package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Target S3 兼容存储目标
type S3Target struct {
	cfg    S3TargetConfig
	client *s3.Client
}

// Configure 配置 S3 目标
func (t *S3Target) Configure(configStr string) error {
	if err := json.Unmarshal([]byte(configStr), &t.cfg); err != nil {
		return fmt.Errorf("解析 S3 配置失败: %w", err)
	}

	if t.cfg.Bucket == "" {
		return fmt.Errorf("S3 Bucket 不能为空")
	}
	if t.cfg.AccessKeyID == "" || t.cfg.SecretAccessKey == "" {
		return fmt.Errorf("S3 凭证不能为空")
	}

	ctx := context.Background()

	// 创建自定义配置
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if t.cfg.Endpoint != "" {
			return aws.Endpoint{
				URL:               t.cfg.Endpoint,
				SigningRegion:     t.cfg.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	region := t.cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			t.cfg.AccessKeyID,
			t.cfg.SecretAccessKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return fmt.Errorf("创建 AWS 配置失败: %w", err)
	}

	// 创建 S3 客户端
	t.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // 兼容 MinIO 等
	})

	return nil
}

// Test 测试 S3 连接
func (t *S3Target) Test() *TargetTestResponse {
	if t.client == nil {
		return &TargetTestResponse{
			Success: false,
			Message: "客户端未初始化",
		}
	}

	ctx := context.Background()

	// 测试 bucket 访问
	_, err := t.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(t.cfg.Bucket),
	})
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "访问 Bucket 失败: " + err.Error(),
		}
	}

	return &TargetTestResponse{
		Success: true,
		Message: "连接成功",
	}
}

// Upload 上传文件到 S3
func (t *S3Target) Upload(localPath, remoteName string, progress func(int)) (string, error) {
	if t.client == nil {
		return "", fmt.Errorf("客户端未初始化")
	}

	ctx := context.Background()

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 构建远程路径
	key := t.getKey(remoteName)

	// 带进度上传
	reader := &ProgressReader{
		Reader:   file,
		Total:    stat.Size(),
		Callback: progress,
	}

	_, err = t.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(t.cfg.Bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(stat.Size()),
	})
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	return key, nil
}

// Download 从 S3 下载文件
func (t *S3Target) Download(remotePath, localPath string, progress func(int)) error {
	if t.client == nil {
		return fmt.Errorf("客户端未初始化")
	}

	ctx := context.Background()

	// 获取对象
	resp, err := t.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.cfg.Bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("获取远程文件失败: %w", err)
	}
	defer resp.Body.Close()

	// 确保本地目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 创建本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer file.Close()

	// 带进度下载
	var total int64
	if resp.ContentLength != nil {
		total = *resp.ContentLength
	}

	progressReader := &ProgressReader{
		Reader:   resp.Body,
		Total:    total,
		Callback: progress,
	}

	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	return nil
}

// Delete 删除 S3 对象
func (t *S3Target) Delete(remotePath string) error {
	if t.client == nil {
		return fmt.Errorf("客户端未初始化")
	}

	ctx := context.Background()

	_, err := t.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(t.cfg.Bucket),
		Key:    aws.String(remotePath),
	})

	return err
}

// List 列出备份文件
func (t *S3Target) List() ([]RemoteFile, error) {
	if t.client == nil {
		return nil, fmt.Errorf("客户端未初始化")
	}

	ctx := context.Background()

	prefix := t.cfg.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	resp, err := t.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(t.cfg.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	var result []RemoteFile
	for _, obj := range resp.Contents {
		if obj.Key == nil {
			continue
		}

		name := *obj.Key
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}

		var size int64
		if obj.Size != nil {
			size = *obj.Size
		}

		var modTime int64
		if obj.LastModified != nil {
			modTime = obj.LastModified.Unix()
		}

		result = append(result, RemoteFile{
			Name:    name,
			Path:    *obj.Key,
			Size:    size,
			ModTime: modTime,
		})
	}

	return result, nil
}

// getKey 获取 S3 对象键
func (t *S3Target) getKey(name string) string {
	if t.cfg.Prefix == "" {
		return name
	}
	prefix := strings.TrimSuffix(t.cfg.Prefix, "/")
	return prefix + "/" + name
}
