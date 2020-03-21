package server

import (
	"fmt"
	"os/exec"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func generateNicName(containerID string) (string, string) {
	return fmt.Sprintf("%s_h", containerID[0:12]), fmt.Sprintf("%s_c", containerID[0:12])
}

func (csh *CNIServerHandler) setNic(podName, podNamespace, netns, containerID, cni0, ip, gateway string) (err error) {
	// 此处我们手动创建veth对, 为了避免与已有设备名称冲突, 这里我们根据containerID生成.
	// 之后将属于容器的veth端移入container, 再将其重命名为eth0(kubelet要求必须要为eth0).
	hostVethName, containerVethName := generateNicName(containerID)

	veth := netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVethName,
			MTU:  1400,
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
		return fmt.Errorf("failed to crate veth for %s %v", podName, err)
	}

	// 将属于宿主机的那一端接入cni0网桥.
	cni0Link, err := netlink.LinkByName(cni0)
	if err != nil {
		return fmt.Errorf("can not find container nic %s %v", hostVethName, err)
	}
	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return fmt.Errorf("can not find container nic %s %v", hostVethName, err)
	}
	err = netlink.LinkSetUp(hostVeth)
	if err != nil {
		return fmt.Errorf("can not set container nic %s up %v", hostVethName, err)
	}
	err = netlink.LinkSetMaster(hostVeth, cni0Link)
	if err != nil {
		return fmt.Errorf("failed to set host veth %s master to %s: %s", hostVethName, cni0, err)
	}

	err = setContainerNic(containerVethName, ip, gateway, netns)
	if err != nil {
		return err
	}

	return nil
}

func (csh *CNIServerHandler) deleteNic(netns, containerID string) error {
	hostVethName, _ := generateNicName(containerID)
	// Remove ovs port
	output, err := exec.Command("ovs-vsctl", "--if-exists", "--with-iface", "del-port", "br-int", hostVethName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete ovs port %v, %s", err, output)
	}

	hostLink, err := netlink.LinkByName(hostVethName)
	if err != nil {
		// If link already not exists, return quietly
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("find host link %s failed %v", hostVethName, err)
	}
	err = netlink.LinkDel(hostLink)
	if err != nil {
		return fmt.Errorf("delete host link %s failed %v", hostLink, err)
	}
	return nil
}

// setContainerNic 容器内部的操作.
func setContainerNic(nicName, ipAddr, gateway, netnsPath string) error {
	containerVeth, err := netlink.LinkByName(nicName)
	if err != nil {
		return fmt.Errorf("can not find container nic %s %v", nicName, err)
	}
	netns, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
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
			return err
		}
		addr, err := netlink.ParseAddr(ipAddr)
		if err != nil {
			return fmt.Errorf("can not parse %s %v", ipAddr, err)
		}
		err = netlink.AddrAdd(containerVeth, addr)
		if err != nil {
			return fmt.Errorf("can not add address to container nic %v", err)
		}

		err = netlink.LinkSetUp(containerVeth)
		if err != nil {
			return fmt.Errorf("can not set container nic %s up %v", nicName, err)
		}

		return nil
	})
}
