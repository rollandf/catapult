#!/bin/bash
echo "Defining VM..."

VM_NAME="fc_host"
LIBVIRT_NETWORK="default"
SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if virsh list --all | grep -q "${VM_NAME}"; then
    echo "${VM_NAME} is already installed... "
else
    dom=$(virt-install --import --name "${VM_NAME}" \
        --memory 1024 --vcpus 1 --cpu host \
        --disk os.img,bus=virtio \
        --os-type=linux \
        --graphics spice \
        --noautoconsole \
        --network=default,model=virtio \
        --connect qemu:///system \
        --print-xml)
    echo $dom | virsh define /dev/stdin
fi

fc_host_status=$(virsh list | grep fc_host | tr -s \"[:blank:]\" | cut -d ' ' -f4)
if [  "${fc_host_status}" == 'running' ]; then
    echo "${VM_NAME} is already running"
    exit 0
fi

ip_address=192.168.122.45
mac_address=$(virsh dumpxml "${VM_NAME}" | grep "mac address" | awk -F\' '{ print $2}')
echo "Setting ip address to ${ip_address} for MAC address ${mac_address}"
"${SCRIPTDIR}"/update_network.py ${VM_NAME} ${LIBVIRT_NETWORK} ${ip_address} ${mac_address}
echo "starting ${VM_NAME}..."
virsh start "${VM_NAME}"
