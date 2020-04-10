// Code generated by protoc-gen-go. DO NOT EDIT.
// source: api.proto

package api

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type DistributeMessageRequest struct {
	Message                 *packet.Publish `protobuf:"bytes,1,opt,name=Message,proto3" json:"Message,omitempty"`
	ResolveRemoteRecipients bool            `protobuf:"varint,2,opt,name=ResolveRemoteRecipients,proto3" json:"ResolveRemoteRecipients,omitempty"`
	XXX_NoUnkeyedLiteral    struct{}        `json:"-"`
	XXX_unrecognized        []byte          `json:"-"`
	XXX_sizecache           int32           `json:"-"`
}

func (m *DistributeMessageRequest) Reset()         { *m = DistributeMessageRequest{} }
func (m *DistributeMessageRequest) String() string { return proto.CompactTextString(m) }
func (*DistributeMessageRequest) ProtoMessage()    {}
func (*DistributeMessageRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{0}
}

func (m *DistributeMessageRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DistributeMessageRequest.Unmarshal(m, b)
}
func (m *DistributeMessageRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DistributeMessageRequest.Marshal(b, m, deterministic)
}
func (m *DistributeMessageRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DistributeMessageRequest.Merge(m, src)
}
func (m *DistributeMessageRequest) XXX_Size() int {
	return xxx_messageInfo_DistributeMessageRequest.Size(m)
}
func (m *DistributeMessageRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DistributeMessageRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DistributeMessageRequest proto.InternalMessageInfo

func (m *DistributeMessageRequest) GetMessage() *packet.Publish {
	if m != nil {
		return m.Message
	}
	return nil
}

func (m *DistributeMessageRequest) GetResolveRemoteRecipients() bool {
	if m != nil {
		return m.ResolveRemoteRecipients
	}
	return false
}

type DistributeMessageResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DistributeMessageResponse) Reset()         { *m = DistributeMessageResponse{} }
func (m *DistributeMessageResponse) String() string { return proto.CompactTextString(m) }
func (*DistributeMessageResponse) ProtoMessage()    {}
func (*DistributeMessageResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{1}
}

func (m *DistributeMessageResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DistributeMessageResponse.Unmarshal(m, b)
}
func (m *DistributeMessageResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DistributeMessageResponse.Marshal(b, m, deterministic)
}
func (m *DistributeMessageResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DistributeMessageResponse.Merge(m, src)
}
func (m *DistributeMessageResponse) XXX_Size() int {
	return xxx_messageInfo_DistributeMessageResponse.Size(m)
}
func (m *DistributeMessageResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_DistributeMessageResponse.DiscardUnknown(m)
}

var xxx_messageInfo_DistributeMessageResponse proto.InternalMessageInfo

type SessionMetadatas struct {
	SessionID            string          `protobuf:"bytes,1,opt,name=SessionID,proto3" json:"SessionID,omitempty"`
	ClientID             string          `protobuf:"bytes,2,opt,name=ClientID,proto3" json:"ClientID,omitempty"`
	ConnectedAt          int64           `protobuf:"varint,3,opt,name=ConnectedAt,proto3" json:"ConnectedAt,omitempty"`
	Peer                 uint64          `protobuf:"varint,4,opt,name=Peer,proto3" json:"Peer,omitempty"`
	LWT                  *packet.Publish `protobuf:"bytes,5,opt,name=LWT,proto3" json:"LWT,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *SessionMetadatas) Reset()         { *m = SessionMetadatas{} }
func (m *SessionMetadatas) String() string { return proto.CompactTextString(m) }
func (*SessionMetadatas) ProtoMessage()    {}
func (*SessionMetadatas) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{2}
}

func (m *SessionMetadatas) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SessionMetadatas.Unmarshal(m, b)
}
func (m *SessionMetadatas) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SessionMetadatas.Marshal(b, m, deterministic)
}
func (m *SessionMetadatas) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SessionMetadatas.Merge(m, src)
}
func (m *SessionMetadatas) XXX_Size() int {
	return xxx_messageInfo_SessionMetadatas.Size(m)
}
func (m *SessionMetadatas) XXX_DiscardUnknown() {
	xxx_messageInfo_SessionMetadatas.DiscardUnknown(m)
}

var xxx_messageInfo_SessionMetadatas proto.InternalMessageInfo

func (m *SessionMetadatas) GetSessionID() string {
	if m != nil {
		return m.SessionID
	}
	return ""
}

func (m *SessionMetadatas) GetClientID() string {
	if m != nil {
		return m.ClientID
	}
	return ""
}

func (m *SessionMetadatas) GetConnectedAt() int64 {
	if m != nil {
		return m.ConnectedAt
	}
	return 0
}

func (m *SessionMetadatas) GetPeer() uint64 {
	if m != nil {
		return m.Peer
	}
	return 0
}

func (m *SessionMetadatas) GetLWT() *packet.Publish {
	if m != nil {
		return m.LWT
	}
	return nil
}

type ListSessionMetadatasRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListSessionMetadatasRequest) Reset()         { *m = ListSessionMetadatasRequest{} }
func (m *ListSessionMetadatasRequest) String() string { return proto.CompactTextString(m) }
func (*ListSessionMetadatasRequest) ProtoMessage()    {}
func (*ListSessionMetadatasRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{3}
}

func (m *ListSessionMetadatasRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSessionMetadatasRequest.Unmarshal(m, b)
}
func (m *ListSessionMetadatasRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSessionMetadatasRequest.Marshal(b, m, deterministic)
}
func (m *ListSessionMetadatasRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSessionMetadatasRequest.Merge(m, src)
}
func (m *ListSessionMetadatasRequest) XXX_Size() int {
	return xxx_messageInfo_ListSessionMetadatasRequest.Size(m)
}
func (m *ListSessionMetadatasRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSessionMetadatasRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ListSessionMetadatasRequest proto.InternalMessageInfo

type ListSessionMetadatasResponse struct {
	SessionMetadatasList []*SessionMetadatas `protobuf:"bytes,1,rep,name=SessionMetadatasList,proto3" json:"SessionMetadatasList,omitempty"`
	XXX_NoUnkeyedLiteral struct{}            `json:"-"`
	XXX_unrecognized     []byte              `json:"-"`
	XXX_sizecache        int32               `json:"-"`
}

func (m *ListSessionMetadatasResponse) Reset()         { *m = ListSessionMetadatasResponse{} }
func (m *ListSessionMetadatasResponse) String() string { return proto.CompactTextString(m) }
func (*ListSessionMetadatasResponse) ProtoMessage()    {}
func (*ListSessionMetadatasResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{4}
}

func (m *ListSessionMetadatasResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSessionMetadatasResponse.Unmarshal(m, b)
}
func (m *ListSessionMetadatasResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSessionMetadatasResponse.Marshal(b, m, deterministic)
}
func (m *ListSessionMetadatasResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSessionMetadatasResponse.Merge(m, src)
}
func (m *ListSessionMetadatasResponse) XXX_Size() int {
	return xxx_messageInfo_ListSessionMetadatasResponse.Size(m)
}
func (m *ListSessionMetadatasResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSessionMetadatasResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ListSessionMetadatasResponse proto.InternalMessageInfo

func (m *ListSessionMetadatasResponse) GetSessionMetadatasList() []*SessionMetadatas {
	if m != nil {
		return m.SessionMetadatasList
	}
	return nil
}

type CreateSubscriptionRequest struct {
	SessionID            string   `protobuf:"bytes,1,opt,name=SessionID,proto3" json:"SessionID,omitempty"`
	Peer                 uint64   `protobuf:"varint,2,opt,name=Peer,proto3" json:"Peer,omitempty"`
	Pattern              []byte   `protobuf:"bytes,3,opt,name=Pattern,proto3" json:"Pattern,omitempty"`
	QoS                  int32    `protobuf:"varint,4,opt,name=QoS,proto3" json:"QoS,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CreateSubscriptionRequest) Reset()         { *m = CreateSubscriptionRequest{} }
func (m *CreateSubscriptionRequest) String() string { return proto.CompactTextString(m) }
func (*CreateSubscriptionRequest) ProtoMessage()    {}
func (*CreateSubscriptionRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{5}
}

func (m *CreateSubscriptionRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CreateSubscriptionRequest.Unmarshal(m, b)
}
func (m *CreateSubscriptionRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CreateSubscriptionRequest.Marshal(b, m, deterministic)
}
func (m *CreateSubscriptionRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CreateSubscriptionRequest.Merge(m, src)
}
func (m *CreateSubscriptionRequest) XXX_Size() int {
	return xxx_messageInfo_CreateSubscriptionRequest.Size(m)
}
func (m *CreateSubscriptionRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CreateSubscriptionRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CreateSubscriptionRequest proto.InternalMessageInfo

func (m *CreateSubscriptionRequest) GetSessionID() string {
	if m != nil {
		return m.SessionID
	}
	return ""
}

func (m *CreateSubscriptionRequest) GetPeer() uint64 {
	if m != nil {
		return m.Peer
	}
	return 0
}

func (m *CreateSubscriptionRequest) GetPattern() []byte {
	if m != nil {
		return m.Pattern
	}
	return nil
}

func (m *CreateSubscriptionRequest) GetQoS() int32 {
	if m != nil {
		return m.QoS
	}
	return 0
}

type CreateSubscriptionResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CreateSubscriptionResponse) Reset()         { *m = CreateSubscriptionResponse{} }
func (m *CreateSubscriptionResponse) String() string { return proto.CompactTextString(m) }
func (*CreateSubscriptionResponse) ProtoMessage()    {}
func (*CreateSubscriptionResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{6}
}

func (m *CreateSubscriptionResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CreateSubscriptionResponse.Unmarshal(m, b)
}
func (m *CreateSubscriptionResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CreateSubscriptionResponse.Marshal(b, m, deterministic)
}
func (m *CreateSubscriptionResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CreateSubscriptionResponse.Merge(m, src)
}
func (m *CreateSubscriptionResponse) XXX_Size() int {
	return xxx_messageInfo_CreateSubscriptionResponse.Size(m)
}
func (m *CreateSubscriptionResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CreateSubscriptionResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CreateSubscriptionResponse proto.InternalMessageInfo

type DeleteSubscriptionRequest struct {
	SessionID            string   `protobuf:"bytes,1,opt,name=SessionID,proto3" json:"SessionID,omitempty"`
	Peer                 uint64   `protobuf:"varint,2,opt,name=Peer,proto3" json:"Peer,omitempty"`
	Pattern              []byte   `protobuf:"bytes,3,opt,name=Pattern,proto3" json:"Pattern,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteSubscriptionRequest) Reset()         { *m = DeleteSubscriptionRequest{} }
func (m *DeleteSubscriptionRequest) String() string { return proto.CompactTextString(m) }
func (*DeleteSubscriptionRequest) ProtoMessage()    {}
func (*DeleteSubscriptionRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{7}
}

func (m *DeleteSubscriptionRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteSubscriptionRequest.Unmarshal(m, b)
}
func (m *DeleteSubscriptionRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteSubscriptionRequest.Marshal(b, m, deterministic)
}
func (m *DeleteSubscriptionRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteSubscriptionRequest.Merge(m, src)
}
func (m *DeleteSubscriptionRequest) XXX_Size() int {
	return xxx_messageInfo_DeleteSubscriptionRequest.Size(m)
}
func (m *DeleteSubscriptionRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteSubscriptionRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteSubscriptionRequest proto.InternalMessageInfo

func (m *DeleteSubscriptionRequest) GetSessionID() string {
	if m != nil {
		return m.SessionID
	}
	return ""
}

func (m *DeleteSubscriptionRequest) GetPeer() uint64 {
	if m != nil {
		return m.Peer
	}
	return 0
}

func (m *DeleteSubscriptionRequest) GetPattern() []byte {
	if m != nil {
		return m.Pattern
	}
	return nil
}

type DeleteSubscriptionResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteSubscriptionResponse) Reset()         { *m = DeleteSubscriptionResponse{} }
func (m *DeleteSubscriptionResponse) String() string { return proto.CompactTextString(m) }
func (*DeleteSubscriptionResponse) ProtoMessage()    {}
func (*DeleteSubscriptionResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{8}
}

func (m *DeleteSubscriptionResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteSubscriptionResponse.Unmarshal(m, b)
}
func (m *DeleteSubscriptionResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteSubscriptionResponse.Marshal(b, m, deterministic)
}
func (m *DeleteSubscriptionResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteSubscriptionResponse.Merge(m, src)
}
func (m *DeleteSubscriptionResponse) XXX_Size() int {
	return xxx_messageInfo_DeleteSubscriptionResponse.Size(m)
}
func (m *DeleteSubscriptionResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteSubscriptionResponse.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteSubscriptionResponse proto.InternalMessageInfo

type ListSubscriptionsResponse struct {
	Subscriptions        []*CreateSubscriptionRequest `protobuf:"bytes,1,rep,name=Subscriptions,proto3" json:"Subscriptions,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                     `json:"-"`
	XXX_unrecognized     []byte                       `json:"-"`
	XXX_sizecache        int32                        `json:"-"`
}

func (m *ListSubscriptionsResponse) Reset()         { *m = ListSubscriptionsResponse{} }
func (m *ListSubscriptionsResponse) String() string { return proto.CompactTextString(m) }
func (*ListSubscriptionsResponse) ProtoMessage()    {}
func (*ListSubscriptionsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{9}
}

func (m *ListSubscriptionsResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSubscriptionsResponse.Unmarshal(m, b)
}
func (m *ListSubscriptionsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSubscriptionsResponse.Marshal(b, m, deterministic)
}
func (m *ListSubscriptionsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSubscriptionsResponse.Merge(m, src)
}
func (m *ListSubscriptionsResponse) XXX_Size() int {
	return xxx_messageInfo_ListSubscriptionsResponse.Size(m)
}
func (m *ListSubscriptionsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSubscriptionsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ListSubscriptionsResponse proto.InternalMessageInfo

func (m *ListSubscriptionsResponse) GetSubscriptions() []*CreateSubscriptionRequest {
	if m != nil {
		return m.Subscriptions
	}
	return nil
}

type ListSubscriptionsRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListSubscriptionsRequest) Reset()         { *m = ListSubscriptionsRequest{} }
func (m *ListSubscriptionsRequest) String() string { return proto.CompactTextString(m) }
func (*ListSubscriptionsRequest) ProtoMessage()    {}
func (*ListSubscriptionsRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{10}
}

func (m *ListSubscriptionsRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSubscriptionsRequest.Unmarshal(m, b)
}
func (m *ListSubscriptionsRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSubscriptionsRequest.Marshal(b, m, deterministic)
}
func (m *ListSubscriptionsRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSubscriptionsRequest.Merge(m, src)
}
func (m *ListSubscriptionsRequest) XXX_Size() int {
	return xxx_messageInfo_ListSubscriptionsRequest.Size(m)
}
func (m *ListSubscriptionsRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSubscriptionsRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ListSubscriptionsRequest proto.InternalMessageInfo

type ShutdownRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ShutdownRequest) Reset()         { *m = ShutdownRequest{} }
func (m *ShutdownRequest) String() string { return proto.CompactTextString(m) }
func (*ShutdownRequest) ProtoMessage()    {}
func (*ShutdownRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{11}
}

func (m *ShutdownRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ShutdownRequest.Unmarshal(m, b)
}
func (m *ShutdownRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ShutdownRequest.Marshal(b, m, deterministic)
}
func (m *ShutdownRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ShutdownRequest.Merge(m, src)
}
func (m *ShutdownRequest) XXX_Size() int {
	return xxx_messageInfo_ShutdownRequest.Size(m)
}
func (m *ShutdownRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ShutdownRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ShutdownRequest proto.InternalMessageInfo

type ShutdownResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ShutdownResponse) Reset()         { *m = ShutdownResponse{} }
func (m *ShutdownResponse) String() string { return proto.CompactTextString(m) }
func (*ShutdownResponse) ProtoMessage()    {}
func (*ShutdownResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00212fb1f9d3bf1c, []int{12}
}

func (m *ShutdownResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ShutdownResponse.Unmarshal(m, b)
}
func (m *ShutdownResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ShutdownResponse.Marshal(b, m, deterministic)
}
func (m *ShutdownResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ShutdownResponse.Merge(m, src)
}
func (m *ShutdownResponse) XXX_Size() int {
	return xxx_messageInfo_ShutdownResponse.Size(m)
}
func (m *ShutdownResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ShutdownResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ShutdownResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*DistributeMessageRequest)(nil), "api.DistributeMessageRequest")
	proto.RegisterType((*DistributeMessageResponse)(nil), "api.DistributeMessageResponse")
	proto.RegisterType((*SessionMetadatas)(nil), "api.SessionMetadatas")
	proto.RegisterType((*ListSessionMetadatasRequest)(nil), "api.ListSessionMetadatasRequest")
	proto.RegisterType((*ListSessionMetadatasResponse)(nil), "api.ListSessionMetadatasResponse")
	proto.RegisterType((*CreateSubscriptionRequest)(nil), "api.CreateSubscriptionRequest")
	proto.RegisterType((*CreateSubscriptionResponse)(nil), "api.CreateSubscriptionResponse")
	proto.RegisterType((*DeleteSubscriptionRequest)(nil), "api.DeleteSubscriptionRequest")
	proto.RegisterType((*DeleteSubscriptionResponse)(nil), "api.DeleteSubscriptionResponse")
	proto.RegisterType((*ListSubscriptionsResponse)(nil), "api.ListSubscriptionsResponse")
	proto.RegisterType((*ListSubscriptionsRequest)(nil), "api.ListSubscriptionsRequest")
	proto.RegisterType((*ShutdownRequest)(nil), "api.ShutdownRequest")
	proto.RegisterType((*ShutdownResponse)(nil), "api.ShutdownResponse")
}

func init() { proto.RegisterFile("api.proto", fileDescriptor_00212fb1f9d3bf1c) }

var fileDescriptor_00212fb1f9d3bf1c = []byte{
	// 560 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x54, 0x4f, 0x73, 0xda, 0x3e,
	0x10, 0xc5, 0x31, 0xf9, 0x05, 0x96, 0x5f, 0x27, 0x44, 0x43, 0xa6, 0xc6, 0x21, 0xa9, 0xe3, 0x13,
	0x3d, 0x04, 0x3a, 0xf4, 0xd2, 0x1e, 0x33, 0x70, 0x61, 0x26, 0x74, 0x88, 0x60, 0x26, 0xa7, 0x1e,
	0x64, 0xb3, 0x03, 0x9a, 0x1a, 0xcb, 0xb1, 0xe4, 0xb4, 0x3d, 0xf5, 0xd3, 0xf4, 0x93, 0xf4, 0x8b,
	0x75, 0xfc, 0xaf, 0xfc, 0xb3, 0xe9, 0xa9, 0x37, 0x69, 0x9f, 0x76, 0xf7, 0xe9, 0x3d, 0x69, 0xa1,
	0xce, 0x02, 0xde, 0x0b, 0x42, 0xa1, 0x04, 0xd1, 0x59, 0xc0, 0xcd, 0x77, 0x4b, 0xae, 0x56, 0x91,
	0xd3, 0x73, 0xc5, 0xba, 0xff, 0xf2, 0xed, 0xce, 0x63, 0x8e, 0xec, 0xaf, 0x9f, 0x95, 0xba, 0x4b,
	0xce, 0xb8, 0xc2, 0xeb, 0x07, 0xcc, 0xfd, 0x82, 0xaa, 0x1f, 0x38, 0x69, 0x9a, 0xfd, 0x03, 0x8c,
	0x11, 0x97, 0x2a, 0xe4, 0x4e, 0xa4, 0x70, 0x82, 0x52, 0xb2, 0x25, 0x52, 0x7c, 0x8e, 0x50, 0x2a,
	0xf2, 0x16, 0xce, 0xb2, 0x88, 0xa1, 0x59, 0x5a, 0xb7, 0x31, 0x38, 0xef, 0xa5, 0xe9, 0xbd, 0x69,
	0xe4, 0x78, 0x5c, 0xae, 0x68, 0x8e, 0x93, 0x0f, 0xf0, 0x9a, 0xa2, 0x14, 0xde, 0x0b, 0x52, 0x5c,
	0x0b, 0x85, 0x14, 0x5d, 0x1e, 0x70, 0xf4, 0x95, 0x34, 0x4e, 0x2c, 0xad, 0x5b, 0xa3, 0x65, 0xb0,
	0x7d, 0x05, 0xed, 0x02, 0x02, 0x32, 0x10, 0xbe, 0x44, 0xfb, 0xa7, 0x06, 0xcd, 0x19, 0x4a, 0xc9,
	0x85, 0x3f, 0x41, 0xc5, 0x16, 0x4c, 0x31, 0x49, 0x3a, 0x50, 0xcf, 0x62, 0xe3, 0x51, 0x42, 0xac,
	0x4e, 0x37, 0x01, 0x62, 0x42, 0x6d, 0xe8, 0xc5, 0xa5, 0xc7, 0xa3, 0xa4, 0x75, 0x9d, 0xfe, 0xd9,
	0x13, 0x0b, 0x1a, 0x43, 0xe1, 0xfb, 0xe8, 0x2a, 0x5c, 0xdc, 0x2b, 0x43, 0xb7, 0xb4, 0xae, 0x4e,
	0xb7, 0x43, 0x84, 0x40, 0x75, 0x8a, 0x18, 0x1a, 0x55, 0x4b, 0xeb, 0x56, 0x69, 0xb2, 0x26, 0xb7,
	0xa0, 0x3f, 0x3c, 0xcd, 0x8d, 0xd3, 0x62, 0x09, 0x62, 0xcc, 0xbe, 0x86, 0xab, 0x07, 0x2e, 0xd5,
	0x3e, 0xd5, 0x4c, 0x48, 0x9b, 0x43, 0xa7, 0x18, 0x4e, 0xaf, 0x49, 0xc6, 0xd0, 0xda, 0xc7, 0xe2,
	0xf3, 0x86, 0x66, 0xe9, 0xdd, 0xc6, 0xe0, 0xb2, 0x17, 0xbb, 0x7c, 0x90, 0x5c, 0x98, 0x62, 0x7f,
	0x87, 0xf6, 0x30, 0x44, 0xa6, 0x70, 0x16, 0x39, 0xd2, 0x0d, 0x79, 0xa0, 0xb8, 0xf0, 0x73, 0x43,
	0x8f, 0x2b, 0x97, 0xdf, 0xfd, 0x64, 0xeb, 0xee, 0x06, 0x9c, 0x4d, 0x99, 0x52, 0x18, 0xfa, 0x89,
	0x5a, 0xff, 0xd3, 0x7c, 0x4b, 0x9a, 0xa0, 0x3f, 0x8a, 0x59, 0x22, 0xd4, 0x29, 0x8d, 0x97, 0x76,
	0x07, 0xcc, 0xa2, 0xd6, 0x99, 0x95, 0x4b, 0x68, 0x8f, 0xd0, 0xc3, 0x7f, 0x4e, 0x2c, 0xa6, 0x51,
	0xd4, 0x28, 0xa3, 0xc1, 0xa0, 0x9d, 0x58, 0xb1, 0x85, 0x6d, 0x7c, 0x18, 0xc1, 0xab, 0x1d, 0x20,
	0x33, 0xe0, 0x26, 0x31, 0xa0, 0x54, 0x56, 0xba, 0x9b, 0x64, 0x9b, 0x60, 0x14, 0xb4, 0x48, 0x5f,
	0xc2, 0x05, 0x9c, 0xcf, 0x56, 0x91, 0x5a, 0x88, 0xaf, 0x79, 0xb6, 0x4d, 0xa0, 0xb9, 0x09, 0xa5,
	0x44, 0x06, 0xbf, 0x74, 0xa8, 0x4e, 0x1e, 0xe7, 0x73, 0x32, 0x87, 0x8b, 0x83, 0x5a, 0xe4, 0x3a,
	0xe1, 0x53, 0xd6, 0xc3, 0xbc, 0x29, 0x83, 0x33, 0x09, 0x2a, 0xe4, 0x09, 0xc8, 0xa1, 0x44, 0x24,
	0xcd, 0x2b, 0x35, 0xc9, 0x7c, 0x53, 0x8a, 0x6f, 0x17, 0x3e, 0x94, 0x89, 0xfc, 0x45, 0xbf, 0xac,
	0xf0, 0x91, 0xb7, 0x53, 0x89, 0x75, 0x38, 0x98, 0x12, 0x99, 0x0e, 0x65, 0xe3, 0x2b, 0xd3, 0xa1,
	0x7c, 0xb8, 0x54, 0xc8, 0x67, 0x68, 0x15, 0xfd, 0x4b, 0x62, 0x6d, 0x14, 0x2c, 0xfe, 0xd1, 0xe6,
	0xed, 0x91, 0x13, 0x79, 0xf9, 0xc1, 0x3d, 0x54, 0x3f, 0x89, 0x05, 0x92, 0x8f, 0x50, 0xcb, 0x1d,
	0x26, 0xad, 0xf4, 0x33, 0xef, 0xbe, 0x01, 0xf3, 0x72, 0x2f, 0x9a, 0x97, 0x70, 0xfe, 0x4b, 0xa6,
	0xf4, 0xfb, 0xdf, 0x01, 0x00, 0x00, 0xff, 0xff, 0x3b, 0x2d, 0x6c, 0xcc, 0xe9, 0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// MQTTClient is the client API for MQTT service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MQTTClient interface {
	ListSubscriptions(ctx context.Context, in *ListSubscriptionsRequest, opts ...grpc.CallOption) (*ListSubscriptionsResponse, error)
	DeleteSubscription(ctx context.Context, in *DeleteSubscriptionRequest, opts ...grpc.CallOption) (*DeleteSubscriptionResponse, error)
	CreateSubscription(ctx context.Context, in *CreateSubscriptionRequest, opts ...grpc.CallOption) (*CreateSubscriptionResponse, error)
	DistributeMessage(ctx context.Context, in *DistributeMessageRequest, opts ...grpc.CallOption) (*DistributeMessageResponse, error)
	ListSessionMetadatas(ctx context.Context, in *ListSessionMetadatasRequest, opts ...grpc.CallOption) (*ListSessionMetadatasResponse, error)
}

type mQTTClient struct {
	cc *grpc.ClientConn
}

func NewMQTTClient(cc *grpc.ClientConn) MQTTClient {
	return &mQTTClient{cc}
}

func (c *mQTTClient) ListSubscriptions(ctx context.Context, in *ListSubscriptionsRequest, opts ...grpc.CallOption) (*ListSubscriptionsResponse, error) {
	out := new(ListSubscriptionsResponse)
	err := c.cc.Invoke(ctx, "/api.MQTT/ListSubscriptions", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mQTTClient) DeleteSubscription(ctx context.Context, in *DeleteSubscriptionRequest, opts ...grpc.CallOption) (*DeleteSubscriptionResponse, error) {
	out := new(DeleteSubscriptionResponse)
	err := c.cc.Invoke(ctx, "/api.MQTT/DeleteSubscription", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mQTTClient) CreateSubscription(ctx context.Context, in *CreateSubscriptionRequest, opts ...grpc.CallOption) (*CreateSubscriptionResponse, error) {
	out := new(CreateSubscriptionResponse)
	err := c.cc.Invoke(ctx, "/api.MQTT/CreateSubscription", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mQTTClient) DistributeMessage(ctx context.Context, in *DistributeMessageRequest, opts ...grpc.CallOption) (*DistributeMessageResponse, error) {
	out := new(DistributeMessageResponse)
	err := c.cc.Invoke(ctx, "/api.MQTT/DistributeMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mQTTClient) ListSessionMetadatas(ctx context.Context, in *ListSessionMetadatasRequest, opts ...grpc.CallOption) (*ListSessionMetadatasResponse, error) {
	out := new(ListSessionMetadatasResponse)
	err := c.cc.Invoke(ctx, "/api.MQTT/ListSessionMetadatas", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MQTTServer is the server API for MQTT service.
type MQTTServer interface {
	ListSubscriptions(context.Context, *ListSubscriptionsRequest) (*ListSubscriptionsResponse, error)
	DeleteSubscription(context.Context, *DeleteSubscriptionRequest) (*DeleteSubscriptionResponse, error)
	CreateSubscription(context.Context, *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error)
	DistributeMessage(context.Context, *DistributeMessageRequest) (*DistributeMessageResponse, error)
	ListSessionMetadatas(context.Context, *ListSessionMetadatasRequest) (*ListSessionMetadatasResponse, error)
}

// UnimplementedMQTTServer can be embedded to have forward compatible implementations.
type UnimplementedMQTTServer struct {
}

func (*UnimplementedMQTTServer) ListSubscriptions(ctx context.Context, req *ListSubscriptionsRequest) (*ListSubscriptionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSubscriptions not implemented")
}
func (*UnimplementedMQTTServer) DeleteSubscription(ctx context.Context, req *DeleteSubscriptionRequest) (*DeleteSubscriptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSubscription not implemented")
}
func (*UnimplementedMQTTServer) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSubscription not implemented")
}
func (*UnimplementedMQTTServer) DistributeMessage(ctx context.Context, req *DistributeMessageRequest) (*DistributeMessageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DistributeMessage not implemented")
}
func (*UnimplementedMQTTServer) ListSessionMetadatas(ctx context.Context, req *ListSessionMetadatasRequest) (*ListSessionMetadatasResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSessionMetadatas not implemented")
}

func RegisterMQTTServer(s *grpc.Server, srv MQTTServer) {
	s.RegisterService(&_MQTT_serviceDesc, srv)
}

func _MQTT_ListSubscriptions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSubscriptionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MQTTServer).ListSubscriptions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.MQTT/ListSubscriptions",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MQTTServer).ListSubscriptions(ctx, req.(*ListSubscriptionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MQTT_DeleteSubscription_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSubscriptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MQTTServer).DeleteSubscription(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.MQTT/DeleteSubscription",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MQTTServer).DeleteSubscription(ctx, req.(*DeleteSubscriptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MQTT_CreateSubscription_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSubscriptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MQTTServer).CreateSubscription(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.MQTT/CreateSubscription",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MQTTServer).CreateSubscription(ctx, req.(*CreateSubscriptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MQTT_DistributeMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DistributeMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MQTTServer).DistributeMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.MQTT/DistributeMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MQTTServer).DistributeMessage(ctx, req.(*DistributeMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MQTT_ListSessionMetadatas_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSessionMetadatasRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MQTTServer).ListSessionMetadatas(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.MQTT/ListSessionMetadatas",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MQTTServer).ListSessionMetadatas(ctx, req.(*ListSessionMetadatasRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _MQTT_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.MQTT",
	HandlerType: (*MQTTServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListSubscriptions",
			Handler:    _MQTT_ListSubscriptions_Handler,
		},
		{
			MethodName: "DeleteSubscription",
			Handler:    _MQTT_DeleteSubscription_Handler,
		},
		{
			MethodName: "CreateSubscription",
			Handler:    _MQTT_CreateSubscription_Handler,
		},
		{
			MethodName: "DistributeMessage",
			Handler:    _MQTT_DistributeMessage_Handler,
		},
		{
			MethodName: "ListSessionMetadatas",
			Handler:    _MQTT_ListSessionMetadatas_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api.proto",
}

// NodeClient is the client API for Node service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type NodeClient interface {
	Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error)
}

type nodeClient struct {
	cc *grpc.ClientConn
}

func NewNodeClient(cc *grpc.ClientConn) NodeClient {
	return &nodeClient{cc}
}

func (c *nodeClient) Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error) {
	out := new(ShutdownResponse)
	err := c.cc.Invoke(ctx, "/api.Node/Shutdown", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NodeServer is the server API for Node service.
type NodeServer interface {
	Shutdown(context.Context, *ShutdownRequest) (*ShutdownResponse, error)
}

// UnimplementedNodeServer can be embedded to have forward compatible implementations.
type UnimplementedNodeServer struct {
}

func (*UnimplementedNodeServer) Shutdown(ctx context.Context, req *ShutdownRequest) (*ShutdownResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Shutdown not implemented")
}

func RegisterNodeServer(s *grpc.Server, srv NodeServer) {
	s.RegisterService(&_Node_serviceDesc, srv)
}

func _Node_Shutdown_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShutdownRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Shutdown(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Node/Shutdown",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Shutdown(ctx, req.(*ShutdownRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Node_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.Node",
	HandlerType: (*NodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Shutdown",
			Handler:    _Node_Shutdown_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api.proto",
}
