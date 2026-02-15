package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

type mockS3Client struct {
	headBucketFn    func(context.Context, *awss3.HeadBucketInput, ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error)
	putObjectFn     func(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	getObjectFn     func(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	deleteObjectFn  func(context.Context, *awss3.DeleteObjectInput, ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
	listObjectsV2Fn func(context.Context, *awss3.ListObjectsV2Input, ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
}

func (m *mockS3Client) HeadBucket(ctx context.Context, in *awss3.HeadBucketInput, optFns ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error) {
	if m.headBucketFn != nil {
		return m.headBucketFn(ctx, in, optFns...)
	}
	return &awss3.HeadBucketOutput{}, nil
}

func (m *mockS3Client) PutObject(ctx context.Context, in *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	if m.putObjectFn != nil {
		return m.putObjectFn(ctx, in, optFns...)
	}
	return &awss3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, in *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	if m.getObjectFn != nil {
		return m.getObjectFn(ctx, in, optFns...)
	}
	return nil, errors.New("unexpected get object")
}

func (m *mockS3Client) DeleteObject(ctx context.Context, in *awss3.DeleteObjectInput, optFns ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	if m.deleteObjectFn != nil {
		return m.deleteObjectFn(ctx, in, optFns...)
	}
	return &awss3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, in *awss3.ListObjectsV2Input, optFns ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	if m.listObjectsV2Fn != nil {
		return m.listObjectsV2Fn(ctx, in, optFns...)
	}
	return &awss3.ListObjectsV2Output{}, nil
}

type mockPresign struct {
	presignGetObjectFn func(context.Context, *awss3.GetObjectInput, ...func(*awss3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

func (m *mockPresign) PresignGetObject(ctx context.Context, in *awss3.GetObjectInput, optFns ...func(*awss3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	if m.presignGetObjectFn != nil {
		return m.presignGetObjectFn(ctx, in, optFns...)
	}
	return nil, errors.New("unexpected presign")
}

func TestNewS3Adapter_Validation(t *testing.T) {
	_, err := NewS3Adapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected validation error for empty bucket/region")
	}
}

func TestUpload_Success(t *testing.T) {
	var gotBucket, gotKey, gotContentType string

	a := &S3Adapter{
		client: &mockS3Client{
			putObjectFn: func(_ context.Context, in *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
				gotBucket = aws.ToString(in.Bucket)
				gotKey = aws.ToString(in.Key)
				gotContentType = aws.ToString(in.ContentType)
				return &awss3.PutObjectOutput{ETag: aws.String(`"etag-1"`)}, nil
			},
		},
		logger: &mockLogger{},
		config: Config{Bucket: "docs", OperationTimeout: time.Second},
	}

	etag, err := a.Upload(context.Background(), "images/a.png", bytes.NewReader([]byte("content")), "image/png", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}
	if etag != "etag-1" {
		t.Fatalf("expected etag-1, got %q", etag)
	}
	if gotBucket != "docs" || gotKey != "images/a.png" || gotContentType != "image/png" {
		t.Fatalf("unexpected put input: bucket=%q key=%q contentType=%q", gotBucket, gotKey, gotContentType)
	}
}

func TestDownload_Success(t *testing.T) {
	a := &S3Adapter{
		client: &mockS3Client{
			getObjectFn: func(_ context.Context, _ *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
				return &awss3.GetObjectOutput{
					Body:        io.NopCloser(bytes.NewReader([]byte("file-content"))),
					ContentType: aws.String("application/pdf"),
				}, nil
			},
		},
		logger: &mockLogger{},
		config: Config{Bucket: "docs"},
	}

	payload, contentType, err := a.Download(context.Background(), "docs/a.pdf")
	if err != nil {
		t.Fatalf("unexpected download error: %v", err)
	}
	if string(payload) != "file-content" {
		t.Fatalf("unexpected payload: %q", string(payload))
	}
	if contentType != "application/pdf" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
}

func TestList_Success(t *testing.T) {
	now := time.Now().UTC()
	a := &S3Adapter{
		client: &mockS3Client{
			listObjectsV2Fn: func(_ context.Context, _ *awss3.ListObjectsV2Input, _ ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
				return &awss3.ListObjectsV2Output{
					Contents: []awss3types.Object{
						{
							Key:          aws.String("images/1.png"),
							ETag:         aws.String(`"a1"`),
							Size:         aws.Int64(128),
							LastModified: aws.Time(now),
						},
					},
				}, nil
			},
		},
		logger: &mockLogger{},
		config: Config{Bucket: "docs"},
	}

	items, err := a.List(context.Background(), "images/", 100)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].Key != "images/1.png" || items[0].ETag != "a1" || items[0].Size != 128 {
		t.Fatalf("unexpected object info: %+v", items[0])
	}
}

func TestPresignGetURL_Success(t *testing.T) {
	a := &S3Adapter{
		client: &mockS3Client{},
		presign: &mockPresign{
			presignGetObjectFn: func(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
				if aws.ToString(in.Key) != "images/1.png" {
					t.Fatalf("unexpected key: %q", aws.ToString(in.Key))
				}
				return &v4.PresignedHTTPRequest{URL: "https://signed.example.com/file"}, nil
			},
		},
		logger: &mockLogger{},
		config: Config{Bucket: "docs", PresignExpiry: time.Minute},
	}

	url, err := a.PresignGetURL(context.Background(), "images/1.png", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected presign error: %v", err)
	}
	if url != "https://signed.example.com/file" {
		t.Fatalf("unexpected url: %q", url)
	}
}

func TestCloseAndHealthCheck_WhenClosed(t *testing.T) {
	a := &S3Adapter{
		client: &mockS3Client{},
		logger: &mockLogger{},
		config: Config{Bucket: "docs"},
	}
	if err := a.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if err := a.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected health check error on closed adapter")
	}
}
