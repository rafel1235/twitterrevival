package media

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type Store interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type StoredObject struct {
	Key      string
	Modified time.Time
	Bytes    int64
}
type ObjectLister interface {
	Objects(context.Context, func(StoredObject) error) error
}
type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3(endpoint, region, bucket, key, secret string) (*S3Store, error) {
	u, err := url.Parse(endpoint)
	local := err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "storage")
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(local && u.Scheme == "http")) || region == "" || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`).MatchString(bucket) || key == "" || len(secret) < 32 {
		return nil, errors.New("invalid S3 configuration: use HTTPS outside loopback/local storage, a bucket and credentials (secret >=32 bytes)")
	}
	client := s3.NewFromConfig(aws.Config{Region: region, Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""), RetryMaxAttempts: 2,
		HTTPClient:                 &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("S3 redirects are disabled") }},
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	return &S3Store{client, bucket}, nil
}
func (s *S3Store) Ready(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	return err
}
func (s *S3Store) Ensure(ctx context.Context) error {
	if s.Ready(ctx) == nil {
		return nil
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket}); err != nil {
		return errors.New("could not initialize private S3 bucket")
	}
	return s.Ready(ctx)
}
func (s *S3Store) Put(ctx context.Context, key, mime string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: bytes.NewReader(data), ContentType: &mime})
	return err
}
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	defer object.Body.Close()
	data, err := io.ReadAll(io.LimitReader(object.Body, MaxStoredBytes+1))
	if err != nil || len(data) > MaxStoredBytes {
		return nil, errors.New("invalid stored image")
	}
	return data, nil
}
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
func (s *S3Store) Objects(ctx context.Context, visit func(StoredObject) error) error {
	pager := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: &s.bucket, Prefix: aws.String("images/")})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, o := range page.Contents {
			if err = visit(StoredObject{aws.ToString(o.Key), aws.ToTime(o.LastModified), aws.ToInt64(o.Size)}); err != nil {
				return err
			}
		}
	}
	return nil
}
