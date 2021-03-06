// Code generated by govpp binapi-generator DO NOT EDIT.
// Package vxlan represents the VPP binary API of the 'vxlan' VPP module.
// Generated from '/usr/share/vpp/api/vxlan.api.json'
package vxlan

import "git.fd.io/govpp.git/api"

// VxlanAddDelTunnel represents the VPP binary API message 'vxlan_add_del_tunnel'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 4:
//
//            "vxlan_add_del_tunnel",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "client_index"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "u8",
//                "is_add"
//            ],
//            [
//                "u8",
//                "is_ipv6"
//            ],
//            [
//                "u32",
//                "instance"
//            ],
//            [
//                "u8",
//                "src_address",
//                16
//            ],
//            [
//                "u8",
//                "dst_address",
//                16
//            ],
//            [
//                "u32",
//                "mcast_sw_if_index"
//            ],
//            [
//                "u32",
//                "encap_vrf_id"
//            ],
//            [
//                "u32",
//                "decap_next_index"
//            ],
//            [
//                "u32",
//                "vni"
//            ],
//            {
//                "crc": "0x00f4bdd0"
//            }
//
type VxlanAddDelTunnel struct {
	IsAdd          uint8
	IsIpv6         uint8
	Instance       uint32
	SrcAddress     []byte `struc:"[16]byte"`
	DstAddress     []byte `struc:"[16]byte"`
	McastSwIfIndex uint32
	EncapVrfID     uint32
	DecapNextIndex uint32
	Vni            uint32
}

func (*VxlanAddDelTunnel) GetMessageName() string {
	return "vxlan_add_del_tunnel"
}
func (*VxlanAddDelTunnel) GetMessageType() api.MessageType {
	return api.RequestMessage
}
func (*VxlanAddDelTunnel) GetCrcString() string {
	return "00f4bdd0"
}
func NewVxlanAddDelTunnel() api.Message {
	return &VxlanAddDelTunnel{}
}

// VxlanAddDelTunnelReply represents the VPP binary API message 'vxlan_add_del_tunnel_reply'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 60:
//
//            "vxlan_add_del_tunnel_reply",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "i32",
//                "retval"
//            ],
//            [
//                "u32",
//                "sw_if_index"
//            ],
//            {
//                "crc": "0xfda5941f"
//            }
//
type VxlanAddDelTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*VxlanAddDelTunnelReply) GetMessageName() string {
	return "vxlan_add_del_tunnel_reply"
}
func (*VxlanAddDelTunnelReply) GetMessageType() api.MessageType {
	return api.ReplyMessage
}
func (*VxlanAddDelTunnelReply) GetCrcString() string {
	return "fda5941f"
}
func NewVxlanAddDelTunnelReply() api.Message {
	return &VxlanAddDelTunnelReply{}
}

// VxlanTunnelDump represents the VPP binary API message 'vxlan_tunnel_dump'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 82:
//
//            "vxlan_tunnel_dump",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "client_index"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "u32",
//                "sw_if_index"
//            ],
//            {
//                "crc": "0x529cb13f"
//            }
//
type VxlanTunnelDump struct {
	SwIfIndex uint32
}

func (*VxlanTunnelDump) GetMessageName() string {
	return "vxlan_tunnel_dump"
}
func (*VxlanTunnelDump) GetMessageType() api.MessageType {
	return api.RequestMessage
}
func (*VxlanTunnelDump) GetCrcString() string {
	return "529cb13f"
}
func NewVxlanTunnelDump() api.Message {
	return &VxlanTunnelDump{}
}

// VxlanTunnelDetails represents the VPP binary API message 'vxlan_tunnel_details'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 104:
//
//            "vxlan_tunnel_details",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "u32",
//                "sw_if_index"
//            ],
//            [
//                "u32",
//                "instance"
//            ],
//            [
//                "u8",
//                "src_address",
//                16
//            ],
//            [
//                "u8",
//                "dst_address",
//                16
//            ],
//            [
//                "u32",
//                "mcast_sw_if_index"
//            ],
//            [
//                "u32",
//                "encap_vrf_id"
//            ],
//            [
//                "u32",
//                "decap_next_index"
//            ],
//            [
//                "u32",
//                "vni"
//            ],
//            [
//                "u8",
//                "is_ipv6"
//            ],
//            {
//                "crc": "0xce38e127"
//            }
//
type VxlanTunnelDetails struct {
	SwIfIndex      uint32
	Instance       uint32
	SrcAddress     []byte `struc:"[16]byte"`
	DstAddress     []byte `struc:"[16]byte"`
	McastSwIfIndex uint32
	EncapVrfID     uint32
	DecapNextIndex uint32
	Vni            uint32
	IsIpv6         uint8
}

func (*VxlanTunnelDetails) GetMessageName() string {
	return "vxlan_tunnel_details"
}
func (*VxlanTunnelDetails) GetMessageType() api.MessageType {
	return api.ReplyMessage
}
func (*VxlanTunnelDetails) GetCrcString() string {
	return "ce38e127"
}
func NewVxlanTunnelDetails() api.Message {
	return &VxlanTunnelDetails{}
}

// SwInterfaceSetVxlanBypass represents the VPP binary API message 'sw_interface_set_vxlan_bypass'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 156:
//
//            "sw_interface_set_vxlan_bypass",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "client_index"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "u32",
//                "sw_if_index"
//            ],
//            [
//                "u8",
//                "is_ipv6"
//            ],
//            [
//                "u8",
//                "enable"
//            ],
//            {
//                "crc": "0xe74ca095"
//            }
//
type SwInterfaceSetVxlanBypass struct {
	SwIfIndex uint32
	IsIpv6    uint8
	Enable    uint8
}

func (*SwInterfaceSetVxlanBypass) GetMessageName() string {
	return "sw_interface_set_vxlan_bypass"
}
func (*SwInterfaceSetVxlanBypass) GetMessageType() api.MessageType {
	return api.RequestMessage
}
func (*SwInterfaceSetVxlanBypass) GetCrcString() string {
	return "e74ca095"
}
func NewSwInterfaceSetVxlanBypass() api.Message {
	return &SwInterfaceSetVxlanBypass{}
}

// SwInterfaceSetVxlanBypassReply represents the VPP binary API message 'sw_interface_set_vxlan_bypass_reply'.
// Generated from '/usr/share/vpp/api/vxlan.api.json', line 186:
//
//            "sw_interface_set_vxlan_bypass_reply",
//            [
//                "u16",
//                "_vl_msg_id"
//            ],
//            [
//                "u32",
//                "context"
//            ],
//            [
//                "i32",
//                "retval"
//            ],
//            {
//                "crc": "0xe8d4e804"
//            }
//
type SwInterfaceSetVxlanBypassReply struct {
	Retval int32
}

func (*SwInterfaceSetVxlanBypassReply) GetMessageName() string {
	return "sw_interface_set_vxlan_bypass_reply"
}
func (*SwInterfaceSetVxlanBypassReply) GetMessageType() api.MessageType {
	return api.ReplyMessage
}
func (*SwInterfaceSetVxlanBypassReply) GetCrcString() string {
	return "e8d4e804"
}
func NewSwInterfaceSetVxlanBypassReply() api.Message {
	return &SwInterfaceSetVxlanBypassReply{}
}
