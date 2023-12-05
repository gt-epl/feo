// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.12.4
// source: offloadproto/offload.proto

package offloadproto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// OffloadStateHubClient is the client API for OffloadStateHub service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OffloadStateHubClient interface {
	UpdateState(ctx context.Context, in *NodeState, opts ...grpc.CallOption) (*UpdateStateResponse, error)
	GetCandidate(ctx context.Context, in *CandidateQuery, opts ...grpc.CallOption) (*CandidateResponse, error)
	GetState(ctx context.Context, in *StateQuery, opts ...grpc.CallOption) (*StateResponse, error)
}

type offloadStateHubClient struct {
	cc grpc.ClientConnInterface
}

func NewOffloadStateHubClient(cc grpc.ClientConnInterface) OffloadStateHubClient {
	return &offloadStateHubClient{cc}
}

func (c *offloadStateHubClient) UpdateState(ctx context.Context, in *NodeState, opts ...grpc.CallOption) (*UpdateStateResponse, error) {
	out := new(UpdateStateResponse)
	err := c.cc.Invoke(ctx, "/offload.OffloadStateHub/UpdateState", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *offloadStateHubClient) GetCandidate(ctx context.Context, in *CandidateQuery, opts ...grpc.CallOption) (*CandidateResponse, error) {
	out := new(CandidateResponse)
	err := c.cc.Invoke(ctx, "/offload.OffloadStateHub/GetCandidate", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *offloadStateHubClient) GetState(ctx context.Context, in *StateQuery, opts ...grpc.CallOption) (*StateResponse, error) {
	out := new(StateResponse)
	err := c.cc.Invoke(ctx, "/offload.OffloadStateHub/GetState", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OffloadStateHubServer is the server API for OffloadStateHub service.
// All implementations must embed UnimplementedOffloadStateHubServer
// for forward compatibility
type OffloadStateHubServer interface {
	UpdateState(context.Context, *NodeState) (*UpdateStateResponse, error)
	GetCandidate(context.Context, *CandidateQuery) (*CandidateResponse, error)
	GetState(context.Context, *StateQuery) (*StateResponse, error)
	mustEmbedUnimplementedOffloadStateHubServer()
}

// UnimplementedOffloadStateHubServer must be embedded to have forward compatible implementations.
type UnimplementedOffloadStateHubServer struct {
}

func (UnimplementedOffloadStateHubServer) UpdateState(context.Context, *NodeState) (*UpdateStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateState not implemented")
}
func (UnimplementedOffloadStateHubServer) GetCandidate(context.Context, *CandidateQuery) (*CandidateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCandidate not implemented")
}
func (UnimplementedOffloadStateHubServer) GetState(context.Context, *StateQuery) (*StateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetState not implemented")
}
func (UnimplementedOffloadStateHubServer) mustEmbedUnimplementedOffloadStateHubServer() {}

// UnsafeOffloadStateHubServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OffloadStateHubServer will
// result in compilation errors.
type UnsafeOffloadStateHubServer interface {
	mustEmbedUnimplementedOffloadStateHubServer()
}

func RegisterOffloadStateHubServer(s grpc.ServiceRegistrar, srv OffloadStateHubServer) {
	s.RegisterService(&OffloadStateHub_ServiceDesc, srv)
}

func _OffloadStateHub_UpdateState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeState)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OffloadStateHubServer).UpdateState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/offload.OffloadStateHub/UpdateState",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OffloadStateHubServer).UpdateState(ctx, req.(*NodeState))
	}
	return interceptor(ctx, in, info, handler)
}

func _OffloadStateHub_GetCandidate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CandidateQuery)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OffloadStateHubServer).GetCandidate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/offload.OffloadStateHub/GetCandidate",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OffloadStateHubServer).GetCandidate(ctx, req.(*CandidateQuery))
	}
	return interceptor(ctx, in, info, handler)
}

func _OffloadStateHub_GetState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StateQuery)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OffloadStateHubServer).GetState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/offload.OffloadStateHub/GetState",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OffloadStateHubServer).GetState(ctx, req.(*StateQuery))
	}
	return interceptor(ctx, in, info, handler)
}

// OffloadStateHub_ServiceDesc is the grpc.ServiceDesc for OffloadStateHub service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OffloadStateHub_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "offload.OffloadStateHub",
	HandlerType: (*OffloadStateHubServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UpdateState",
			Handler:    _OffloadStateHub_UpdateState_Handler,
		},
		{
			MethodName: "GetCandidate",
			Handler:    _OffloadStateHub_GetCandidate_Handler,
		},
		{
			MethodName: "GetState",
			Handler:    _OffloadStateHub_GetState_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "offloadproto/offload.proto",
}
