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

// InitiateMultipartUploadResult is the response for InitiateMultipartUpload
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

// CompleteMultipartUploadResult is the response for CompleteMultipartUpload
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// CompleteMultipartUpload is the request body for CompleteMultipartUpload
type CompleteMultipartUpload struct {
	XMLName xml.Name       `xml:"CompleteMultipartUpload"`
	Parts   []CompletePart `xml:"Part"`
}

// CompletePart represents a part in the CompleteMultipartUpload request
type CompletePart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// ListPartsResult is the response for ListParts
type ListPartsResult struct {
	XMLName              xml.Name  `xml:"ListPartsResult"`
	Bucket               string    `xml:"Bucket"`
	Key                  string    `xml:"Key"`
	UploadID             string    `xml:"UploadId"`
	Initiator            Initiator `xml:"Initiator"`
	Owner                Owner     `xml:"Owner"`
	StorageClass         string    `xml:"StorageClass"`
	PartNumberMarker     int       `xml:"PartNumberMarker"`
	NextPartNumberMarker int       `xml:"NextPartNumberMarker"`
	MaxParts             int       `xml:"MaxParts"`
	IsTruncated          bool      `xml:"IsTruncated"`
	Parts                []Part    `xml:"Part"`
}

// Initiator represents the initiator of a multipart upload
type Initiator struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// Part represents a part in a multipart upload
type Part struct {
	PartNumber   int    `xml:"PartNumber"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

// ListMultipartUploadsResult is the response for ListMultipartUploads
type ListMultipartUploadsResult struct {
	XMLName            xml.Name `xml:"ListMultipartUploadsResult"`
	Bucket             string   `xml:"Bucket"`
	KeyMarker          string   `xml:"KeyMarker"`
	UploadIDMarker     string   `xml:"UploadIdMarker"`
	NextKeyMarker      string   `xml:"NextKeyMarker"`
	NextUploadIDMarker string   `xml:"NextUploadIdMarker"`
	MaxUploads         int      `xml:"MaxUploads"`
	IsTruncated        bool     `xml:"IsTruncated"`
	Uploads            []Upload `xml:"Upload"`
}

// Upload represents a multipart upload in progress
type Upload struct {
	Key          string    `xml:"Key"`
	UploadID     string    `xml:"UploadId"`
	Initiator    Initiator `xml:"Initiator"`
	Owner        Owner     `xml:"Owner"`
	StorageClass string    `xml:"StorageClass"`
	Initiated    string    `xml:"Initiated"`
}
