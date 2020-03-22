package util

const (
	// IPAddressAnnotation 点分十进制+掩码字符串, 如`192.168.0.1/24`.
	// 必须同时指定gateway的地址.
	IPAddressAnnotation  = "ovn.kubernetes.io/ip_address"
	// GatewayAnnotation 点分十进制+掩码字符串, 如`192.168.0.254`
	// 必须处于IPAddressAnnotation所指的网络中.
	GatewayAnnotation    = "ovn.kubernetes.io/gateway"
	IPPoolAnnotation     = "ovn.kubernetes.io/ip_pool"
)
