/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

type RequestParametersSpec struct {
	SourceIPAddress string `json:"sourceIPAddress"`
}

type ResponseElementsSpec struct {
	XAmzRequestId string `json:"x-amz-request-id"`
	XAmzId2       string `json:"x-amz-id-2"`
}

type UserIdentitySpec struct {
	PrincipalId string `json:"principalId"`
}

type OwnerdentitySpec struct {
	PrincipalId string `json:"principalId"`
}

type BucketSpec struct {
	Name          string           `json:"name"`
	OwnerIdentity OwnerdentitySpec `json:"ownerIdentity"`
	Arn           string           `json:"arn"`
	Id            string           `json:"id"`
}

type MetadataEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ObjectSpec struct {
	Key       string          `json:"key"`
	Size      uint            `json:"size"`
	ETag      string          `json:"eTag"`
	VersionId string          `json:"versionId"`
	Sequencer string          `json:"sequencer"`
	Metadata  []MetadataEntry `json:"metadata"`
}

type S3Spec struct {
	S3SchemaVersion string     `json:"s3SchemaVersion"`
	ConfigurationId string     `json:"configurationId"`
	Bucket          BucketSpec `json:"bucket"`
	Object          ObjectSpec `json:"object"`
}

type BucketNotification struct {
	EventVersion      string                `json:"eventVersion"`
	EventSource       string                `json:"eventSource"`
	AwsRegion         string                `json:"awsRegion"`
	EventTime         string                `json:"eventTime"`
	EventName         string                `json:"eventName"`
	UserIdentity      UserIdentitySpec      `json:"userIdentity"`
	RequestParameters RequestParametersSpec `json:"requestParameters"`
	ResponseElements  ResponseElementsSpec  `json:"responseElements"`
	S3                S3Spec                `json:"s3"`
	EventId           string                `json:"eventId"`
}

type BucketNotifications struct {
	Records []BucketNotification `json:"Records"`
}
