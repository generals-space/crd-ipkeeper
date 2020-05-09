package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimtypes "k8s.io/apimachinery/pkg/types"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StaticIP describes a StaticIP resource
type StaticIP struct {
	// TypeMeta为各资源通用元信息, 包括kind和apiVersion.
	metav1.TypeMeta `json:",inline"`
	// ObjectMeta为特定类型的元信息, 包括name, namespace, selfLink, labels等.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// spec字段
	Spec StaticIPSpec `json:"spec"`
	// status字段
	Status StaticIPStatus `json:"status"`
}

// StaticIPSpec is the spec for a MyResource resource
type StaticIPSpec struct {
	Namespace string `json:"namespace"`
	OwnerKind string `json:"ownerKind"`
	// 格式可为 "192.168.1.1/24,192.168.1.2/24"
	IPPool  string `json:"ipPool"`
	Gateway string `json:"gateway"`

	// IPMap key 为 192.168.1.1/24 这种点分十进制字符串
	// val 为 OwnerPod 对象, 表示此 IP 的拥有者
	IPMap     map[string]*OwnerPod `json:"ipmap"`
	Used      []string             `json:"used"`
	Avaliable []string             `json:"avaliable"`
	// 已分配的IP占IP池的比例, 如 1/4, 2/4 等
	Ratio string `json:"ratio"`
}

// OwnerPod ...
type OwnerPod struct {
	Namespace string        `json:"namespace"`
	Name      string        `json:"name"`
	UID       apimtypes.UID `json:"uid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StaticIPList is a list of StaticIP resources
type StaticIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []StaticIP `json:"items"`
}

// StaticIPStatus is the status for a StaticIPStatus resource
type StaticIPStatus struct{}
