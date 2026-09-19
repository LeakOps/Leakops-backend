package services

import(
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)


type StorageService struct {
	client 			*s3.Client
	bucketName		string
	publicURL		string
}


func NewsStorageService(endpoint, region, accessKeyID, secretKey, bucketName, publicURL string) *StorageService {
	client := s3.New(s3.Options{
		BaseEndpoint:  	aws.String(endpoint),
		Region: 		region,
		Credentials: 	credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
		UsePathStyle: 	true,  // Supabase S3-compatible endpoint needs path-style URLs
	})

	return &StorageService{
		client: 	client,
		bucketName: bucketName,
		publicURL: 	strings.TrimSuffix(publicURL, "/"),
	}
}


// UploadFile uploads a file to Supabase Storage and returns its public URL.
func (s *StorageService) UploadFile(ctx context.Context, key string, file io.Reader, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: 		aws.String(s.bucketName),
		Key: 			aws.String(key),
		Body: 			file,
		ContentType: 	aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file %w", err)
	}

	return fmt.Sprintf("%s/%s", s.publicURL, key), nil
}
