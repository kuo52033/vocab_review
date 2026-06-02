package audios

import (
	"bytes"
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"vocabreview/backend/internal/domain"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3Storage(config aws.Config, bucket string) *S3Storage {
	return &S3Storage{client: s3.NewFromConfig(config), bucket: bucket}
}

func (s *S3Storage) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Storage) Bucket() string {
	return s.bucket
}

func storageKey(job domain.VocabAudioJob) string {
	return "audio/" + sanitizePath(job.Provider) + "/" + sanitizePath(job.Model) + "/" + sanitizePath(job.Voice) + "/" + job.InputHash + "." + sanitizePath(job.OutputFormat)
}

func audioID(inputHash string) string {
	return "aud_" + inputHash
}

func sanitizePath(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '.', char == '_', char == '-':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}
