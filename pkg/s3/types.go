package s3

import "encoding/xml"

// ListBucketsResponse is the response for ListBuckets operation
type ListBucketsResponse struct {
	XMLName xml.Name   `xml:"ListAllMyBucketsResult"`
	Owner   Owner      `xml:"Owner"`
	Buckets BucketList `xml:"Buckets"`
}

// BucketList contains a list of buckets
type BucketList struct {
	Buckets []Bucket `xml:"Bucket"`
}

// Bucket represents a bucket
type Bucket struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

// Owner represents the bucket owner
type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// ListObjectsResponse is the response for ListObjects operation
type ListObjectsResponse struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix,omitempty"`
	Delimiter      string         `xml:"Delimiter,omitempty"`
	MaxKeys        int            `xml:"MaxKeys"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []Object       `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// Object represents an S3 object
type Object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// CommonPrefix represents a common prefix in listing
type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// ErrorResponse is the response for errors
type ErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}
