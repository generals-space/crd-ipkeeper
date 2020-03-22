package server

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
	"github.com/vishvananda/netlink"
)

func generateVethName(containerID string) (string, string) {
	return fmt.Sprintf("%s_h", containerID[0:12]), fmt.Sprintf("%s_c", containerID[0:12])
}

// setVethPair 创建并设置veth pair对, 一端连接cni网桥, 一端放入pod容器.
// err结果以fmt.Errorf()形式返回, 此函数中并不输出.
func (csh *CNIServerHandler) setVethPair(podReq *restapi.PodRequest, ipAddr, gateway string) (err error) {
	// 此处我们手动创建veth对, 为了避免与已有设备名称冲突, 这里我们根据containerID生成.
	// 之后将属于容器的veth端移入container, 再将其重命名为eth0(kubelet要求必须要为eth0).
	hostVethName, containerVethName := generateVethName(podReq.ContainerID)
	veth := netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVethName,
		},
		PeerName: containerVethName,
	}
	defer func() {
		// Remove veth link in case any error during creating pod network.
		if err != nil {
			netlink.LinkDel(&veth)
		}
	}()
	err = netlink.LinkAdd(&veth)
	if err != nil {
		return fmt.Errorf("failed to create veth pair for pod %s %v", podReq.PodName, err)
	}

	err = setHostVeth(hostVethName, podReq.CNI0)
	if err != nil {
		return err
	}

	err = setContainerVeth(containerVethName, ipAddr, gateway, podReq.NetNs)
	if err != nil {
		return err
	}

	return nil
}

// setHostVeth 设置宿主机上的设备, 将属于宿主机的那一端接入cni0网桥并启动等.
func setHostVeth(vethName, cni0 string) (err error) {
	// 将属于宿主机的那一端接入cni0网桥.
	cni0Link, err := netlink.LinkByName(cni0)
	if err != nil {
		return fmt.Errorf("failed to find bridge device %s: %s", cni0, err)
	}
	hostVeth, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("failed to find host veth %s: %s", vethName, err)
	}
	err = netlink.LinkSetUp(hostVeth)
	if err != nil {
		return fmt.Errorf("can not set host veth %s up: %s", vethName, err)
	}
	err = netlink.LinkSetMaster(hostVeth, cni0Link)
	if err != nil {
		return fmt.Errorf("failed to set host veth %s master to %s: %s", vethName, cni0, err)
	}
	return nil
}

// setContainerVeth 容器内部的操作.
func setContainerVeth(vethName, ipAddr, gateway, netnsPath string) error {
	containerVeth, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("can not find container nic %s %v", vethName, err)
	}
	netns, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %s: %v", netnsPath, err)
	}
	// 把属于容器的那一端veth通过netns放进去.
	err = netlink.LinkSetNsFd(containerVeth, int(netns.Fd()))
	if err != nil {
		return fmt.Errorf("failed to link netns %v", err)
	}

	return ns.WithNetNSPath(netns.Path(), func(_ ns.NetNS) error {
		// 把veth pair在容器端的设备名修改为eth0, 否则kubelet会重建此Pod.
		err = netlink.LinkSetName(containerVeth, "eth0")
		if err != nil {
			return fmt.Errorf("failed to rename container veth %s: %s", ipAddr, err)
		}
		addr, err := netlink.ParseAddr(ipAddr)
		if err != nil {
			return fmt.Errorf("can not parse ip address %s: %s", ipAddr, err)
		}
		err = netlink.AddrAdd(containerVeth, addr)
		if err != nil {
			return fmt.Errorf("can not add address %s to container eth0: %s", ipAddr, err)
		}

		err = netlink.LinkSetUp(containerVeth)
		if err != nil {
			return fmt.Errorf("can not set container eth0 %s up %s", ipAddr, err)
		}
		// 设置默认路由, 此操作应该是cni插件中的ipam部分完成的, 这里我们需要手动添加.
		_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
		err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: containerVeth.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       defNet,
			Gw:        net.ParseIP(gateway),
		})
		if err != nil {
			return fmt.Errorf("failed to add route for %s: %s", ipAddr, err)
		}

		return nil
	})
}
