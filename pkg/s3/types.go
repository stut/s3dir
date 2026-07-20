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

// ListObjectsResponse is the response for the ListObjects (v1) operation
type ListObjectsResponse struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix,omitempty"`
	Delimiter      string         `xml:"Delimiter,omitempty"`
	Marker         string         `xml:"Marker"`
	NextMarker     string         `xml:"NextMarker,omitempty"`
	MaxKeys        int            `xml:"MaxKeys"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []Object       `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// ListObjectsV2Response is the response for the ListObjectsV2 operation
type ListObjectsV2Response struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix,omitempty"`
	Delimiter             string         `xml:"Delimiter,omitempty"`
	StartAfter            string         `xml:"StartAfter,omitempty"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []Object       `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
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

// LocationConstraint is the response for GetBucketLocation. An empty value
// means the default region (us-east-1), matching AWS behaviour
type LocationConstraint struct {
	XMLName xml.Name `xml:"LocationConstraint"`
	Value   string   `xml:",chardata"`
}

// VersioningConfiguration is the response for GetBucketVersioning. An empty
// configuration means versioning has never been enabled
type VersioningConfiguration struct {
	XMLName xml.Name `xml:"VersioningConfiguration"`
}

// AccessControlPolicy is the response for GetBucketAcl and GetObjectAcl
type AccessControlPolicy struct {
	XMLName           xml.Name          `xml:"AccessControlPolicy"`
	Owner             Owner             `xml:"Owner"`
	AccessControlList AccessControlList `xml:"AccessControlList"`
}

// AccessControlList contains the grants of an access control policy
type AccessControlList struct {
	Grants []Grant `xml:"Grant"`
}

// Grant represents a single ACL grant
type Grant struct {
	Grantee    Grantee `xml:"Grantee"`
	Permission string  `xml:"Permission"`
}

// Grantee identifies who a grant applies to
type Grantee struct {
	XMLNSXSI    string `xml:"xmlns:xsi,attr"`
	Type        string `xml:"xsi:type,attr"`
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// Tagging is the response for GetObjectTagging
type Tagging struct {
	XMLName xml.Name `xml:"Tagging"`
	TagSet  TagSet   `xml:"TagSet"`
}

// TagSet contains the tags of a tagging configuration
type TagSet struct {
	Tags []Tag `xml:"Tag"`
}

// Tag is a single key/value tag
type Tag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// CopyObjectResult is the response for CopyObject
type CopyObjectResult struct {
	XMLName      xml.Name `xml:"CopyObjectResult"`
	LastModified string   `xml:"LastModified"`
	ETag         string   `xml:"ETag"`
}

// CopyPartResult is the response for UploadPartCopy
type CopyPartResult struct {
	XMLName      xml.Name `xml:"CopyPartResult"`
	LastModified string   `xml:"LastModified"`
	ETag         string   `xml:"ETag"`
}

// Delete is the request body for DeleteObjects
type Delete struct {
	XMLName xml.Name           `xml:"Delete"`
	Quiet   bool               `xml:"Quiet"`
	Objects []ObjectIdentifier `xml:"Object"`
}

// ObjectIdentifier identifies an object in a DeleteObjects request
type ObjectIdentifier struct {
	Key string `xml:"Key"`
}

// DeleteResult is the response for DeleteObjects
type DeleteResult struct {
	XMLName xml.Name        `xml:"DeleteResult"`
	Deleted []DeletedObject `xml:"Deleted"`
	Errors  []DeleteError   `xml:"Error"`
}

// DeletedObject represents a successfully deleted object
type DeletedObject struct {
	Key string `xml:"Key"`
}

// DeleteError represents a failed deletion in a DeleteObjects response
type DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
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
