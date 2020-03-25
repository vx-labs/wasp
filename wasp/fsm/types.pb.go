// Code generated by protoc-gen-go. DO NOT EDIT.
// source: types.proto

package fsm

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
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

type RetainedMessageStored struct {
	Publish              *packet.Publish `protobuf:"bytes,1,opt,name=Publish,proto3" json:"Publish,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *RetainedMessageStored) Reset()         { *m = RetainedMessageStored{} }
func (m *RetainedMessageStored) String() string { return proto.CompactTextString(m) }
func (*RetainedMessageStored) ProtoMessage()    {}
func (*RetainedMessageStored) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{0}
}

func (m *RetainedMessageStored) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RetainedMessageStored.Unmarshal(m, b)
}
func (m *RetainedMessageStored) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RetainedMessageStored.Marshal(b, m, deterministic)
}
func (m *RetainedMessageStored) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RetainedMessageStored.Merge(m, src)
}
func (m *RetainedMessageStored) XXX_Size() int {
	return xxx_messageInfo_RetainedMessageStored.Size(m)
}
func (m *RetainedMessageStored) XXX_DiscardUnknown() {
	xxx_messageInfo_RetainedMessageStored.DiscardUnknown(m)
}

var xxx_messageInfo_RetainedMessageStored proto.InternalMessageInfo

func (m *RetainedMessageStored) GetPublish() *packet.Publish {
	if m != nil {
		return m.Publish
	}
	return nil
}

type RetainedMessageDeleted struct {
	Topic                []byte   `protobuf:"bytes,1,opt,name=Topic,proto3" json:"Topic,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RetainedMessageDeleted) Reset()         { *m = RetainedMessageDeleted{} }
func (m *RetainedMessageDeleted) String() string { return proto.CompactTextString(m) }
func (*RetainedMessageDeleted) ProtoMessage()    {}
func (*RetainedMessageDeleted) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{1}
}

func (m *RetainedMessageDeleted) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RetainedMessageDeleted.Unmarshal(m, b)
}
func (m *RetainedMessageDeleted) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RetainedMessageDeleted.Marshal(b, m, deterministic)
}
func (m *RetainedMessageDeleted) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RetainedMessageDeleted.Merge(m, src)
}
func (m *RetainedMessageDeleted) XXX_Size() int {
	return xxx_messageInfo_RetainedMessageDeleted.Size(m)
}
func (m *RetainedMessageDeleted) XXX_DiscardUnknown() {
	xxx_messageInfo_RetainedMessageDeleted.DiscardUnknown(m)
}

var xxx_messageInfo_RetainedMessageDeleted proto.InternalMessageInfo

func (m *RetainedMessageDeleted) GetTopic() []byte {
	if m != nil {
		return m.Topic
	}
	return nil
}

type SubscriptionCreated struct {
	SessionID            string   `protobuf:"bytes,1,opt,name=SessionID,proto3" json:"SessionID,omitempty"`
	Peer                 uint64   `protobuf:"varint,2,opt,name=Peer,proto3" json:"Peer,omitempty"`
	Pattern              []byte   `protobuf:"bytes,3,opt,name=Pattern,proto3" json:"Pattern,omitempty"`
	Qos                  int32    `protobuf:"varint,4,opt,name=Qos,proto3" json:"Qos,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SubscriptionCreated) Reset()         { *m = SubscriptionCreated{} }
func (m *SubscriptionCreated) String() string { return proto.CompactTextString(m) }
func (*SubscriptionCreated) ProtoMessage()    {}
func (*SubscriptionCreated) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{2}
}

func (m *SubscriptionCreated) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SubscriptionCreated.Unmarshal(m, b)
}
func (m *SubscriptionCreated) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SubscriptionCreated.Marshal(b, m, deterministic)
}
func (m *SubscriptionCreated) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SubscriptionCreated.Merge(m, src)
}
func (m *SubscriptionCreated) XXX_Size() int {
	return xxx_messageInfo_SubscriptionCreated.Size(m)
}
func (m *SubscriptionCreated) XXX_DiscardUnknown() {
	xxx_messageInfo_SubscriptionCreated.DiscardUnknown(m)
}

var xxx_messageInfo_SubscriptionCreated proto.InternalMessageInfo

func (m *SubscriptionCreated) GetSessionID() string {
	if m != nil {
		return m.SessionID
	}
	return ""
}

func (m *SubscriptionCreated) GetPeer() uint64 {
	if m != nil {
		return m.Peer
	}
	return 0
}

func (m *SubscriptionCreated) GetPattern() []byte {
	if m != nil {
		return m.Pattern
	}
	return nil
}

func (m *SubscriptionCreated) GetQos() int32 {
	if m != nil {
		return m.Qos
	}
	return 0
}

type SubscriptionDeleted struct {
	SessionID            string   `protobuf:"bytes,1,opt,name=SessionID,proto3" json:"SessionID,omitempty"`
	Pattern              []byte   `protobuf:"bytes,2,opt,name=Pattern,proto3" json:"Pattern,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SubscriptionDeleted) Reset()         { *m = SubscriptionDeleted{} }
func (m *SubscriptionDeleted) String() string { return proto.CompactTextString(m) }
func (*SubscriptionDeleted) ProtoMessage()    {}
func (*SubscriptionDeleted) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{3}
}

func (m *SubscriptionDeleted) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SubscriptionDeleted.Unmarshal(m, b)
}
func (m *SubscriptionDeleted) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SubscriptionDeleted.Marshal(b, m, deterministic)
}
func (m *SubscriptionDeleted) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SubscriptionDeleted.Merge(m, src)
}
func (m *SubscriptionDeleted) XXX_Size() int {
	return xxx_messageInfo_SubscriptionDeleted.Size(m)
}
func (m *SubscriptionDeleted) XXX_DiscardUnknown() {
	xxx_messageInfo_SubscriptionDeleted.DiscardUnknown(m)
}

var xxx_messageInfo_SubscriptionDeleted proto.InternalMessageInfo

func (m *SubscriptionDeleted) GetSessionID() string {
	if m != nil {
		return m.SessionID
	}
	return ""
}

func (m *SubscriptionDeleted) GetPattern() []byte {
	if m != nil {
		return m.Pattern
	}
	return nil
}

type PeerLost struct {
	Peer                 uint64   `protobuf:"varint,1,opt,name=Peer,proto3" json:"Peer,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PeerLost) Reset()         { *m = PeerLost{} }
func (m *PeerLost) String() string { return proto.CompactTextString(m) }
func (*PeerLost) ProtoMessage()    {}
func (*PeerLost) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{4}
}

func (m *PeerLost) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PeerLost.Unmarshal(m, b)
}
func (m *PeerLost) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PeerLost.Marshal(b, m, deterministic)
}
func (m *PeerLost) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeerLost.Merge(m, src)
}
func (m *PeerLost) XXX_Size() int {
	return xxx_messageInfo_PeerLost.Size(m)
}
func (m *PeerLost) XXX_DiscardUnknown() {
	xxx_messageInfo_PeerLost.DiscardUnknown(m)
}

var xxx_messageInfo_PeerLost proto.InternalMessageInfo

func (m *PeerLost) GetPeer() uint64 {
	if m != nil {
		return m.Peer
	}
	return 0
}

type StateTransitionSet struct {
	Events               []*StateTransition `protobuf:"bytes,1,rep,name=events,proto3" json:"events,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *StateTransitionSet) Reset()         { *m = StateTransitionSet{} }
func (m *StateTransitionSet) String() string { return proto.CompactTextString(m) }
func (*StateTransitionSet) ProtoMessage()    {}
func (*StateTransitionSet) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{5}
}

func (m *StateTransitionSet) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StateTransitionSet.Unmarshal(m, b)
}
func (m *StateTransitionSet) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StateTransitionSet.Marshal(b, m, deterministic)
}
func (m *StateTransitionSet) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StateTransitionSet.Merge(m, src)
}
func (m *StateTransitionSet) XXX_Size() int {
	return xxx_messageInfo_StateTransitionSet.Size(m)
}
func (m *StateTransitionSet) XXX_DiscardUnknown() {
	xxx_messageInfo_StateTransitionSet.DiscardUnknown(m)
}

var xxx_messageInfo_StateTransitionSet proto.InternalMessageInfo

func (m *StateTransitionSet) GetEvents() []*StateTransition {
	if m != nil {
		return m.Events
	}
	return nil
}

type StateTransition struct {
	// Types that are valid to be assigned to Event:
	//	*StateTransition_RetainedMessageStored
	//	*StateTransition_RetainedMessageDeleted
	//	*StateTransition_SessionSubscribed
	//	*StateTransition_SessionUnsubscribed
	//	*StateTransition_PeerLost
	Event                isStateTransition_Event `protobuf_oneof:"Event"`
	XXX_NoUnkeyedLiteral struct{}                `json:"-"`
	XXX_unrecognized     []byte                  `json:"-"`
	XXX_sizecache        int32                   `json:"-"`
}

func (m *StateTransition) Reset()         { *m = StateTransition{} }
func (m *StateTransition) String() string { return proto.CompactTextString(m) }
func (*StateTransition) ProtoMessage()    {}
func (*StateTransition) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{6}
}

func (m *StateTransition) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StateTransition.Unmarshal(m, b)
}
func (m *StateTransition) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StateTransition.Marshal(b, m, deterministic)
}
func (m *StateTransition) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StateTransition.Merge(m, src)
}
func (m *StateTransition) XXX_Size() int {
	return xxx_messageInfo_StateTransition.Size(m)
}
func (m *StateTransition) XXX_DiscardUnknown() {
	xxx_messageInfo_StateTransition.DiscardUnknown(m)
}

var xxx_messageInfo_StateTransition proto.InternalMessageInfo

type isStateTransition_Event interface {
	isStateTransition_Event()
}

type StateTransition_RetainedMessageStored struct {
	RetainedMessageStored *RetainedMessageStored `protobuf:"bytes,1,opt,name=RetainedMessageStored,proto3,oneof"`
}

type StateTransition_RetainedMessageDeleted struct {
	RetainedMessageDeleted *RetainedMessageDeleted `protobuf:"bytes,2,opt,name=RetainedMessageDeleted,proto3,oneof"`
}

type StateTransition_SessionSubscribed struct {
	SessionSubscribed *SubscriptionCreated `protobuf:"bytes,3,opt,name=SessionSubscribed,proto3,oneof"`
}

type StateTransition_SessionUnsubscribed struct {
	SessionUnsubscribed *SubscriptionDeleted `protobuf:"bytes,4,opt,name=SessionUnsubscribed,proto3,oneof"`
}

type StateTransition_PeerLost struct {
	PeerLost *PeerLost `protobuf:"bytes,5,opt,name=PeerLost,proto3,oneof"`
}

func (*StateTransition_RetainedMessageStored) isStateTransition_Event() {}

func (*StateTransition_RetainedMessageDeleted) isStateTransition_Event() {}

func (*StateTransition_SessionSubscribed) isStateTransition_Event() {}

func (*StateTransition_SessionUnsubscribed) isStateTransition_Event() {}

func (*StateTransition_PeerLost) isStateTransition_Event() {}

func (m *StateTransition) GetEvent() isStateTransition_Event {
	if m != nil {
		return m.Event
	}
	return nil
}

func (m *StateTransition) GetRetainedMessageStored() *RetainedMessageStored {
	if x, ok := m.GetEvent().(*StateTransition_RetainedMessageStored); ok {
		return x.RetainedMessageStored
	}
	return nil
}

func (m *StateTransition) GetRetainedMessageDeleted() *RetainedMessageDeleted {
	if x, ok := m.GetEvent().(*StateTransition_RetainedMessageDeleted); ok {
		return x.RetainedMessageDeleted
	}
	return nil
}

func (m *StateTransition) GetSessionSubscribed() *SubscriptionCreated {
	if x, ok := m.GetEvent().(*StateTransition_SessionSubscribed); ok {
		return x.SessionSubscribed
	}
	return nil
}

func (m *StateTransition) GetSessionUnsubscribed() *SubscriptionDeleted {
	if x, ok := m.GetEvent().(*StateTransition_SessionUnsubscribed); ok {
		return x.SessionUnsubscribed
	}
	return nil
}

func (m *StateTransition) GetPeerLost() *PeerLost {
	if x, ok := m.GetEvent().(*StateTransition_PeerLost); ok {
		return x.PeerLost
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*StateTransition) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*StateTransition_RetainedMessageStored)(nil),
		(*StateTransition_RetainedMessageDeleted)(nil),
		(*StateTransition_SessionSubscribed)(nil),
		(*StateTransition_SessionUnsubscribed)(nil),
		(*StateTransition_PeerLost)(nil),
	}
}

func init() {
	proto.RegisterType((*RetainedMessageStored)(nil), "fsm.RetainedMessageStored")
	proto.RegisterType((*RetainedMessageDeleted)(nil), "fsm.RetainedMessageDeleted")
	proto.RegisterType((*SubscriptionCreated)(nil), "fsm.SubscriptionCreated")
	proto.RegisterType((*SubscriptionDeleted)(nil), "fsm.SubscriptionDeleted")
	proto.RegisterType((*PeerLost)(nil), "fsm.PeerLost")
	proto.RegisterType((*StateTransitionSet)(nil), "fsm.StateTransitionSet")
	proto.RegisterType((*StateTransition)(nil), "fsm.StateTransition")
}

func init() { proto.RegisterFile("types.proto", fileDescriptor_d938547f84707355) }

var fileDescriptor_d938547f84707355 = []byte{
	// 420 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x53, 0xef, 0x6e, 0xd3, 0x30,
	0x10, 0x6f, 0x96, 0x76, 0x65, 0x17, 0xd0, 0xc0, 0x1b, 0xc8, 0x1a, 0x08, 0x55, 0xf9, 0x54, 0x04,
	0x4b, 0x51, 0x79, 0x83, 0x32, 0xa4, 0x20, 0x6d, 0x52, 0x71, 0xb6, 0x07, 0x70, 0xd2, 0xdb, 0x66,
	0xd1, 0xc6, 0x21, 0x77, 0x9d, 0xe0, 0x15, 0x78, 0x6a, 0x14, 0x37, 0x59, 0x58, 0xd7, 0xec, 0x9b,
	0x7d, 0xfe, 0xfd, 0xb3, 0x7f, 0x09, 0x04, 0xfc, 0xa7, 0x40, 0x8a, 0x8a, 0xd2, 0xb2, 0x15, 0xfe,
	0x35, 0xad, 0x4e, 0x3e, 0xdf, 0x18, 0xbe, 0x5d, 0xa7, 0x51, 0x66, 0x57, 0x93, 0xbb, 0xdf, 0xa7,
	0x4b, 0x9d, 0xd2, 0x64, 0xf5, 0x8b, 0xf9, 0xd4, 0x61, 0x32, 0xbb, 0x9c, 0x14, 0x3a, 0xfb, 0x89,
	0x3c, 0x29, 0xd2, 0x0d, 0x2d, 0x9c, 0xc1, 0x6b, 0x85, 0xac, 0x4d, 0x8e, 0x8b, 0x0b, 0x24, 0xd2,
	0x37, 0x98, 0xb0, 0x2d, 0x71, 0x21, 0x3e, 0xc0, 0x70, 0xbe, 0x4e, 0x97, 0x86, 0x6e, 0xa5, 0x37,
	0xf2, 0xc6, 0xc1, 0xf4, 0x30, 0xda, 0x70, 0xa3, 0x7a, 0xac, 0x9a, 0xf3, 0x30, 0x82, 0x37, 0x5b,
	0x1a, 0x67, 0xb8, 0x44, 0xc6, 0x85, 0x38, 0x86, 0xc1, 0xa5, 0x2d, 0x4c, 0xe6, 0x24, 0x9e, 0xab,
	0xcd, 0x26, 0x24, 0x38, 0x4a, 0xd6, 0x29, 0x65, 0xa5, 0x29, 0xd8, 0xd8, 0xfc, 0x6b, 0x89, 0xba,
	0x02, 0xbf, 0x83, 0x83, 0x04, 0x89, 0x8c, 0xcd, 0xbf, 0x9f, 0x39, 0xc2, 0x81, 0x6a, 0x07, 0x42,
	0x40, 0x7f, 0x8e, 0x58, 0xca, 0xbd, 0x91, 0x37, 0xee, 0x2b, 0xb7, 0x16, 0x12, 0x86, 0x73, 0xcd,
	0x8c, 0x65, 0x2e, 0x7d, 0x67, 0xd0, 0x6c, 0xc5, 0x4b, 0xf0, 0x7f, 0x58, 0x92, 0xfd, 0x91, 0x37,
	0x1e, 0xa8, 0x6a, 0x19, 0x5e, 0x3c, 0x34, 0x6d, 0x12, 0x3e, 0x6d, 0xfa, 0x9f, 0xc1, 0xde, 0x03,
	0x83, 0xf0, 0x3d, 0x3c, 0xab, 0x22, 0x9c, 0x5b, 0xe2, 0xfb, 0x68, 0x5e, 0x1b, 0x2d, 0x9c, 0x81,
	0x48, 0x58, 0x33, 0x5e, 0x96, 0x3a, 0x27, 0x53, 0x39, 0x26, 0xc8, 0xe2, 0x13, 0xec, 0xe3, 0x1d,
	0xe6, 0x4c, 0xd2, 0x1b, 0xf9, 0xe3, 0x60, 0x7a, 0x1c, 0x5d, 0xd3, 0x2a, 0xda, 0x02, 0xaa, 0x1a,
	0x13, 0xfe, 0xf5, 0xe1, 0x70, 0xeb, 0x4c, 0xa8, 0x8e, 0xbe, 0xea, 0x92, 0x4e, 0x9c, 0xe0, 0x4e,
	0x44, 0xdc, 0x53, 0x1d, 0x55, 0x5f, 0x75, 0xf5, 0xe7, 0x2e, 0x1d, 0x4c, 0xdf, 0xee, 0x12, 0xad,
	0x21, 0x71, 0x4f, 0x75, 0x95, 0x1f, 0xc3, 0xab, 0xfa, 0x25, 0xeb, 0x87, 0x4f, 0x71, 0xe1, 0x7a,
	0x0a, 0xa6, 0x72, 0x73, 0xef, 0xc7, 0x1f, 0x41, 0xdc, 0x53, 0x8f, 0x49, 0xe2, 0x1c, 0x8e, 0xea,
	0xe1, 0x55, 0x4e, 0xad, 0x56, 0xbf, 0x43, 0xab, 0x8d, 0xb6, 0x8b, 0x26, 0x3e, 0xb6, 0xd5, 0xc9,
	0x81, 0x93, 0x78, 0xe1, 0x24, 0x9a, 0x61, 0xdc, 0x53, 0xf7, 0x80, 0xd9, 0x10, 0x06, 0xdf, 0xaa,
	0x36, 0xd2, 0x7d, 0xf7, 0xbf, 0x7c, 0xf9, 0x17, 0x00, 0x00, 0xff, 0xff, 0xd9, 0xa7, 0x4b, 0x30,
	0x75, 0x03, 0x00, 0x00,
}