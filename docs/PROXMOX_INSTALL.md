# Installing Nevatal Cloud on Proxmox VE (Dell PowerEdge R640)

This guide installs the Nevatal Cloud control plane (Go API, React web UI,
PostgreSQL) on a Proxmox VE host and connects it to the hypervisor through the
Proxmox REST API. It covers Proxmox VE **8.x** and **9.x**, and two run modes:

| Mode | Purpose | Hypervisor | Accounts |
|---|---|---|---|
| **demo** | Public showcase (e.g. `demo.nevatal.id`) | Simulated, in memory. Never touches Proxmox. | Public demo accounts with published passwords |
| **production** | Real service (e.g. `cloud.nevatal.id`) | Your Proxmox VE node, through a least-privilege API token | One bootstrap admin from `ADMIN_EMAIL` / `ADMIN_PASSWORD` |

Production can also let customers publish a port of their VM at
`https://<name>.nevatal.id`, Railway-style, through a Cloudflare Tunnel: no
public IP and no port forwarding needed (section 20).

The AI Infrastructure Orchestrator (plans, approvals and durable executions
behind the AI Cloud Agent) runs inside the API replicas and uses the token
from section 4; its user and operator guide is [ORCHESTRATOR.md](ORCHESTRATOR.md).

Commands are copy-pasteable. Replace the example values from the table below
with your own. Lines marked **(not verified here)** were written from the
upstream documentation but could not be executed while writing this guide.

## Contents

1. [Example addresses used in this guide](#1-example-addresses-used-in-this-guide)
2. [Choose a topology](#2-choose-a-topology)
3. [Prerequisites](#3-prerequisites)
4. [Proxmox API user, role and token](#4-proxmox-api-user-role-and-token)
5. [Cloud-init templates 9000-9003](#5-cloud-init-templates-9000-9003)
6. [Create the control-plane VM](#6-create-the-control-plane-vm)
7. [Install Docker Engine](#7-install-docker-engine)
8. [Get the code](#8-get-the-code)
9. [Demo deployment](#9-demo-deployment)
10. [Production deployment](#10-production-deployment)
11. [TLS / HTTPS options](#11-tls--https-options)
12. [Firewall](#12-firewall)
13. [Verify the Proxmox integration and the Docker viewer](#13-verify-the-proxmox-integration-and-the-docker-viewer)
14. [Migrating from demo to production](#14-migrating-from-demo-to-production)
15. [Backups and restore](#15-backups-and-restore)
16. [Upgrades and rollback](#16-upgrades-and-rollback)
17. [Load balancing](#17-load-balancing)
18. [Topology B: several VMs behind HAProxy + keepalived](#18-topology-b-several-vms-behind-haproxy--keepalived)
19. [Topology C: k3s cluster](#19-topology-c-k3s-cluster)
20. [Cloudflare Tunnel and Railway-style subdomains (*.nevatal.id)](#20-cloudflare-tunnel-and-railway-style-subdomains-nevatalid)
21. [Troubleshooting](#21-troubleshooting)

---

## 1. Example addresses used in this guide

| What | Example value |
|---|---|
| Proxmox VE host (R640) | `10.0.0.2`, node name `pve-dell-r640`, bridge `vmbr0`, gateway `10.0.0.1` |
| VM storage | `local-zfs` |
| Admin workstations | `10.0.10.0/24` |
| Control-plane VM (topology A) | VMID `150`, `10.0.0.50` |
| Topology B | LB VMs `10.0.0.5` / `10.0.0.6`, VIP `10.0.0.10`, app VMs `10.0.0.11` / `10.0.0.12`, PostgreSQL VM `10.0.0.20`, NFS VM `10.0.0.21` |
| Topology C | k3s VMs `10.0.0.31-33`, MetalLB pool `10.0.0.240-10.0.0.250` |
| Customer VM network (section 20) | VLAN `20` on `vmbr0`, `10.20.0.0/24`, gateway `10.20.0.1`; the control-plane VM's second NIC `10.20.0.5` |
| Public names | `cloud.nevatal.id` (production), `demo.nevatal.id` (demo), `<name>.nevatal.id` (customer apps; a separate domain such as `<name>.nevatal.app` is recommended, section 20.2) |
| Customer VM IDs | assigned by Proxmox (`/cluster/nextid`), templates use `9000-9003` |

---

## 2. Choose a topology

**A. One control-plane VM with Docker Compose (recommended).**
Everything (nginx, API replicas, PostgreSQL, optional Caddy for HTTPS) runs in
one VM on the R640. Simple to operate, backed up with one `vzdump`. The API
still runs several replicas behind nginx for zero-effort restarts and CPU
headroom.

```
Internet --443--> [VM 150: Caddy -> nginx -> API x2 -> PostgreSQL]
                                         |
                                         +--8006--> Proxmox VE API (R640)
```

**A + Cloudflare Tunnel (home lab / colo without a public IP).** The same VM,
but `cloudflared` dials out to Cloudflare, so no inbound port is opened.
Cloudflare terminates HTTPS for the control plane and, with the optional
subdomain feature, for every customer app at `<name>.nevatal.id`, which the
`edge` proxy forwards to the customer VM's private IP and port (section 20):

```
Browser --HTTPS--> Cloudflare (DNS cloud.nevatal.id + *.nevatal.id, TLS, WAF)
                      ^
                      | tunnel, opened from inside (outbound TCP/UDP 7844)
[VM 150: cloudflared -+-> nginx :8080 -> API x2 -> PostgreSQL     cloud.nevatal.id
                      +-> edge  :8088 --net1 (VLAN 20)--> customer VM 10.20.0.x:PORT
                                                          <name>.nevatal.id
         API --8006--> Proxmox VE API (R640) ]
```

**B. Several VMs behind HAProxy + keepalived.**
Two small load-balancer VMs share a virtual IP (VRRP), N app VMs run nginx +
API replicas, one PostgreSQL VM and one NFS VM hold the shared state. Survives
the loss of one app VM or one LB VM. Use it when one VM is not enough or you
need to patch app VMs without downtime. All VMs still live on the same R640, so
this protects against VM/software failures, not against losing the host.

```
Internet -> VIP 10.0.0.10 (lb1/lb2 HAProxy, keepalived)
              -> app1 :3080 (nginx -> API x2) -+-> PostgreSQL VM
              -> app2 :3080 (nginx -> API x2) -+-> NFS VM (/data/storage)
```

**C. k3s cluster in VMs (MetalLB + Traefik).**
Kubernetes manifests (kustomize) with autoscaling, PodDisruptionBudgets and
NetworkPolicies. Choose it if you already run Kubernetes or want declarative
rolling updates. More moving parts than A or B.

Demo mode always runs **one** API replica (the simulated hypervisor keeps its
state in process memory), so topology A is the right choice for a demo.

---

## 3. Prerequisites

**Hardware (Dell PowerEdge R640)**

- BIOS: *Processor Settings -> Virtualization Technology* enabled, *Integrated
  Devices -> SR-IOV Global Enable* optional. VT-d is needed only for PCI
  passthrough.
- Storage controller in HBA/pass-through mode (e.g. PERC H330, or H730P/H740P
  switched to HBA mode) when you use ZFS (`local-zfs`).
- iDRAC 9 reachable from your admin network: it is your console if a firewall
  change locks you out.

**Proxmox VE**

- Proxmox VE 8.x (Debian 12) or 9.x (Debian 13) installed, web UI on
  `https://10.0.0.2:8006`, root shell (SSH or *Node -> Shell*).
- A VM bridge (`vmbr0`) with **a DHCP server on that network**: Nevatal
  configures customer VMs with `ipconfig0=ip=dhcp` and reads the address back
  through the QEMU guest agent.
- Internet access from the host (cloud images) and from the control-plane VM
  (Docker images, Let's Encrypt).
- Check the version and node name:

```bash
pveversion
hostname          # = PROXMOX_NODE_NAME, e.g. pve-dell-r640
pvesm status      # storage IDs, e.g. local-zfs
```

**DNS / network**

- A DNS `A` record for the public name (`cloud.nevatal.id` / `demo.nevatal.id`)
  pointing to your public IP, and TCP 80 + TCP/UDP 443 forwarded to the
  control-plane VM (or the VIP / MetalLB address in topologies B and C).
  **No public IP?** Use a Cloudflare Tunnel instead: the zone on Cloudflare,
  outbound TCP/UDP 7844 and HTTPS from the control-plane VM, nothing inbound
  (section 20).
- Recommended: keep customer VMs on a different VLAN or bridge than the
  Proxmox management address. Customer VMs must not reach port 8006.

---

## 4. Proxmox API user, role and token

Nevatal talks to Proxmox with an **API token** (`PVEAPIToken=USER@REALM!TOKENID=SECRET`).
Give it only the privileges it uses. The table lists every API call the
control plane makes and the privilege Proxmox checks (taken from the
`qemu-server` / `pve-manager` source of PVE 8 and PVE 9).

| Feature | API call | Privilege (PVE 8) | Privilege (PVE 9) | ACL path |
|---|---|---|---|---|
| Node telemetry | `GET /nodes/{node}/status` | `Sys.Audit` | `Sys.Audit` | `/nodes/{node}` |
| Storage list | `GET /nodes/{node}/storage` | `Datastore.Audit` | `Datastore.Audit` | `/storage/{storage}` |
| Next free VMID | `GET /cluster/nextid` | any authenticated user | same | - |
| Create VM | `POST .../qemu/{template}/clone` (full clone) | `VM.Clone` (template), `VM.Allocate` (new VM), `Datastore.AllocateSpace`, `SDN.Use` (bridge) | same | `/vms`, `/storage/{storage}`, `/sdn/zones/localnetwork/{bridge}` |
| CPU / RAM / cloud-init / agent flag | `POST .../config` | `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Cloudinit`, `VM.Config.Options` | same | `/vms` |
| Grow disk | `PUT .../resize` | `VM.Config.Disk` + `Datastore.AllocateSpace` | same | `/vms`, `/storage/{storage}` |
| Start / stop / shutdown / reboot | `POST .../status/*` | `VM.PowerMgmt` | same | `/vms` |
| VM status | `GET .../status/current` | `VM.Audit` | same | `/vms` |
| Delete VM (`purge=1`) | `DELETE .../qemu/{vmid}` | `VM.Allocate` | same | `/vms` |
| Snapshots | create/delete, rollback, list | `VM.Snapshot`, `VM.Snapshot.Rollback`, `VM.Audit` | same (9.2+: starting after a rollback also checks `VM.PowerMgmt`) | `/vms` |
| Web console | `POST .../termproxy` + websocket | `VM.Console` | same | `/vms` |
| Task progress | `GET /nodes/{node}/tasks/{upid}/status` | own tasks; `Sys.Audit` for others | same | `/nodes/{node}` |
| IP address discovery | `GET .../agent/network-get-interfaces` | `VM.Monitor` | `VM.GuestAgent.Audit` | `/vms` |
| **Docker viewer** | `POST .../agent/exec`, `GET .../agent/exec-status` | `VM.Monitor` | `VM.GuestAgent.Unrestricted` | `/vms` |

PVE 9.0 removed `VM.Monitor` and split guest-agent access into
`VM.GuestAgent.Audit`, `VM.GuestAgent.FileRead`, `VM.GuestAgent.FileWrite`,
`VM.GuestAgent.FileSystemMgmt` and `VM.GuestAgent.Unrestricted` (Proxmox VE
9.0 release notes; `PVE/API2/Qemu/Agent.pm` in qemu-server: `exec` and
`exec-status` require `VM.GuestAgent.Unrestricted`). Each version rejects the
other's privilege names, so use the block for your version.

> **Security warning - the Docker viewer.** `agent/exec` runs any command as
> **root inside every customer VM** the token can reach. On PVE 8 this power is
> part of `VM.Monitor`, which Nevatal also needs to read VM IP addresses. On
> PVE 9 it is the separate `VM.GuestAgent.Unrestricted` privilege: grant it only
> if you want the Docker viewer, and protect `.env` (it contains the token)
> like a root password.

### 4.1 Create the role

Run as root on the Proxmox host.

**Proxmox VE 9.x**

```bash
pveum role add NevatalCloud --privs "VM.Allocate VM.Audit VM.Clone VM.Config.CPU VM.Config.Cloudinit VM.Config.Disk VM.Config.Memory VM.Config.Options VM.Console VM.PowerMgmt VM.Snapshot VM.Snapshot.Rollback VM.GuestAgent.Audit Datastore.Audit Datastore.AllocateSpace Sys.Audit SDN.Use"

# Optional: Docker viewer (arbitrary command execution in guests, see warning above)
pveum role add NevatalGuestExec --privs "VM.GuestAgent.Unrestricted"
```

**Proxmox VE 8.x**

```bash
pveum role add NevatalCloud --privs "VM.Allocate VM.Audit VM.Clone VM.Config.CPU VM.Config.Cloudinit VM.Config.Disk VM.Config.Memory VM.Config.Options VM.Console VM.PowerMgmt VM.Snapshot VM.Snapshot.Rollback VM.Monitor Datastore.Audit Datastore.AllocateSpace Sys.Audit SDN.Use"
```

Upgrading a host from PVE 8 to 9 later: `pve8to9` warns about roles using
`VM.Monitor`, which the upgrade drops. Re-add guest-agent access afterwards:

```bash
pveum role modify NevatalCloud --append 1 --privs "VM.GuestAgent.Audit"
pveum role add NevatalGuestExec --privs "VM.GuestAgent.Unrestricted"   # if you use the Docker viewer
```

### 4.2 Create the user, the token and the ACLs

```bash
NODE=pve-dell-r640        # hostname of the R640 node
STORAGE=local-zfs         # PROXMOX_STORAGE
BRIDGE=vmbr0              # bridge used by the templates' net0

pveum user add nevatal@pve --comment "Nevatal Cloud control plane"

# Privilege-separated token (default). Copy the "value" it prints: it is shown ONCE.
pveum user token add nevatal@pve control-plane --privsep 1 --comment "Nevatal Cloud API"
```

With privilege separation, the token's effective rights are the
**intersection** of the user's and the token's ACLs, so every ACL is granted to
both. The token id contains `!`: keep it in **single quotes** (an interactive
bash expands `!` inside double quotes).

```bash
TOKEN_ID='nevatal@pve!control-plane'
for path in /vms "/storage/$STORAGE" "/nodes/$NODE" "/sdn/zones/localnetwork/$BRIDGE"; do
  pveum acl modify "$path" --users nevatal@pve --roles NevatalCloud
  pveum acl modify "$path" --tokens "$TOKEN_ID" --roles NevatalCloud
done

# Docker viewer, PVE 9 only:
# pveum acl modify /vms --users nevatal@pve --roles NevatalGuestExec
# pveum acl modify /vms --tokens "$TOKEN_ID" --roles NevatalGuestExec
```

`VM.Allocate` must sit on `/vms` (all VMs), because Nevatal does not put new
VMs into a pool and their VMIDs are not known in advance. That also covers
your infrastructure VMs and the templates. Restrict them on the user: an ACL on
a deeper path replaces the inherited ones, `NoAccess` cancels all roles, and
the token can never exceed its user.

```bash
# Infrastructure VMs (control plane, LB/app/DB VMs, k3s nodes): no access.
pveum acl modify /vms/150 --users nevatal@pve --roles NoAccess

# Templates: clone only (cannot be changed, started or deleted by Nevatal).
pveum role add NevatalTemplate --privs "VM.Audit VM.Clone"
for vmid in 9000 9001 9002 9003; do
  pveum acl modify "/vms/$vmid" --users nevatal@pve --roles NevatalTemplate
done
```

Check the result:

```bash
pveum acl list
pveum user token permissions nevatal@pve control-plane
```

### 4.3 Test the token from outside Proxmox

`pvesh` and `qm` run as `root@pam` and prove nothing about the token. Test with
the token itself, ideally from the control-plane VM (after section 6):

```bash
PVE=https://10.0.0.2:8006/api2/json
TOKEN='nevatal@pve!control-plane=00000000-0000-0000-0000-000000000000'   # id=secret

curl -sk -H "Authorization: PVEAPIToken=$TOKEN" "$PVE/version"
curl -sk -H "Authorization: PVEAPIToken=$TOKEN" "$PVE/nodes/pve-dell-r640/status" | head -c 300; echo
curl -sk -H "Authorization: PVEAPIToken=$TOKEN" "$PVE/nodes/pve-dell-r640/storage"  | head -c 300; echo
```

`401` means a wrong token id/secret; `403 Permission check failed (...)` names
the missing privilege and path.

---

## 5. Cloud-init templates 9000-9003

Nevatal provisions every VM as a **full clone** of a template, then sets CPU,
memory, `ciuser`, `sshkeys`, `ipconfig0=ip=dhcp`, `agent=1` and grows `scsi0`.
The seeded catalog expects these templates:

| VMID | Catalog entry | Image | Default user |
|---|---|---|---|
| 9000 | Ubuntu Server 24.04 LTS | `noble-server-cloudimg-amd64.img` | `ubuntu` |
| 9001 | Debian 12 / 13 | `debian-12-genericcloud-amd64.qcow2` | `debian` |
| 9002 | Alpine Linux 3.20 | `nocloud_alpine-3.20.10-x86_64-bios-cloudinit-r0.qcow2` | `alpine` |
| 9003 | WordPress on Ubuntu 24.04 (LEMP) | Ubuntu 24.04 + nginx, PHP-FPM, MariaDB, WordPress | `ubuntu` |

Template rules that Nevatal relies on:

- Boot disk on **`scsi0`**, **20 GiB** (the API grows it by `plan disk - 20`).
- Cloud-init drive (`ide2`), serial console, `--agent enabled=1`.
- **qemu-guest-agent installed inside the image**: Nevatal reads the VM's IP
  address and runs the Docker viewer through it.
- Empty `/etc/machine-id`, so clones do not share DHCP leases.
- Never boot a template VM. Test with a clone (section 5.4).

### 5.1 Tools and images

```bash
apt update && apt install -y libguestfs-tools
mkdir -p /var/lib/vz/template/nevatal && cd /var/lib/vz/template/nevatal

wget -O ubuntu-2404.img   https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
wget -O debian-12.qcow2   https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2
wget -O alpine-320.qcow2  https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/cloud/nocloud_alpine-3.20.10-x86_64-bios-cloudinit-r0.qcow2
```

Alpine 3.20 is past its end of support; the catalog name comes from the
seed data. Any newer `nocloud_alpine-3.2x...-bios-cloudinit` image works the
same way.

### 5.2 Install qemu-guest-agent into the images

```bash
cd /var/lib/vz/template/nevatal

virt-customize -a ubuntu-2404.img --install qemu-guest-agent --truncate /etc/machine-id
virt-customize -a debian-12.qcow2 --install qemu-guest-agent --truncate /etc/machine-id

# Alpine (OpenRC) - (not verified here)
virt-customize -a alpine-320.qcow2 \
  --run-command 'apk add --no-cache qemu-guest-agent && rc-update add qemu-guest-agent default'
```

On Ubuntu and Debian the agent service starts automatically when the VM has
the agent device (`--agent enabled=1`). `virt-customize` needs internet access
from the Proxmox host to download the packages.

### 5.3 Create the templates

Define a helper in the root shell, then create each template:

```bash
STORAGE=local-zfs
BRIDGE=vmbr0          # add ",tag=<vlan>" in --net0 below for a customer VLAN

make_template() {   # make_template VMID NAME IMAGE
  qm create "$1" --name "$2" --ostype l26 \
    --cores 2 --memory 2048 --cpu host \
    --net0 "virtio,bridge=$BRIDGE" \
    --scsihw virtio-scsi-single \
    --scsi0 "$STORAGE:0,import-from=$3,discard=on,ssd=1" \
    --ide2 "$STORAGE:cloudinit" \
    --boot order=scsi0 \
    --serial0 socket --vga serial0 \
    --agent enabled=1
  qm disk resize "$1" scsi0 20G
  qm template "$1"
}

make_template 9000 tpl-ubuntu-2404 /var/lib/vz/template/nevatal/ubuntu-2404.img
make_template 9001 tpl-debian-12   /var/lib/vz/template/nevatal/debian-12.qcow2
make_template 9002 tpl-alpine-320  /var/lib/vz/template/nevatal/alpine-320.qcow2
```

`import-from` needs Proxmox VE 7.2 or newer (PVE 8 and 9 both have it).

**9003 - WordPress on Ubuntu 24.04 (example, not verified here).** The image
gets nginx, PHP-FPM, MariaDB and the latest WordPress. Database credentials
are generated on each clone's first boot, so no secret is baked into the
template:

```bash
cd /var/lib/vz/template/nevatal
cp ubuntu-2404.img wordpress-2404.img      # already contains qemu-guest-agent
qemu-img resize wordpress-2404.img 10G

cat > wp-firstboot.sh <<'EOF'
#!/bin/sh
set -e
DBPASS=$(openssl rand -hex 16)
mysql -e "CREATE DATABASE IF NOT EXISTS wordpress; CREATE USER IF NOT EXISTS 'wordpress'@'localhost' IDENTIFIED BY '$DBPASS'; GRANT ALL ON wordpress.* TO 'wordpress'@'localhost'; FLUSH PRIVILEGES;"
cd /var/www/wordpress
cp wp-config-sample.php wp-config.php
sed -i "s/database_name_here/wordpress/; s/username_here/wordpress/; s/password_here/$DBPASS/" wp-config.php
curl -fsSL https://api.wordpress.org/secret-key/1.1/salt/ >> wp-config.php
chown -R www-data:www-data /var/www/wordpress
PHPSOCK=$(ls /run/php/php*-fpm.sock | head -n1)
cat > /etc/nginx/sites-available/wordpress <<NGINX
server {
    listen 80 default_server;
    root /var/www/wordpress;
    index index.php;
    client_max_body_size 64m;
    location / { try_files \$uri \$uri/ /index.php?\$args; }
    location ~ \.php\$ { include snippets/fastcgi-php.conf; fastcgi_pass unix:$PHPSOCK; }
}
NGINX
ln -sf /etc/nginx/sites-available/wordpress /etc/nginx/sites-enabled/default
systemctl reload nginx
EOF

virt-customize -a wordpress-2404.img \
  --install nginx,mariadb-server,php-fpm,php-mysql,php-curl,php-gd,php-intl,php-mbstring,php-xml,php-zip,curl,openssl \
  --run-command 'curl -fsSL https://wordpress.org/latest.tar.gz | tar -xz -C /var/www' \
  --upload wp-firstboot.sh:/usr/local/sbin/nevatal-wp-firstboot.sh \
  --chmod 0755:/usr/local/sbin/nevatal-wp-firstboot.sh \
  --firstboot-command /usr/local/sbin/nevatal-wp-firstboot.sh \
  --truncate /etc/machine-id

make_template 9003 tpl-wordpress-2404 /var/lib/vz/template/nevatal/wordpress-2404.img
```

**Optional: Docker pre-installed.** Customers who want the Docker viewer need
Docker in the guest. A template with Docker already installed (register it in
the admin UI under a new VMID, e.g. 9010):

```bash
cp ubuntu-2404.img ubuntu-2404-docker.img
virt-customize -a ubuntu-2404-docker.img --install docker.io,docker-compose-v2 --truncate /etc/machine-id
make_template 9010 tpl-ubuntu-2404-docker /var/lib/vz/template/nevatal/ubuntu-2404-docker.img
```

### 5.4 Test a template with a throw-away clone

```bash
qm clone 9000 990 --name tpl-test --full 1 --storage local-zfs
qm set 990 --ciuser ubuntu --sshkeys ~/.ssh/authorized_keys --ipconfig0 ip=dhcp
qm start 990
sleep 60
qm guest cmd 990 ping && echo "guest agent OK"
qm guest cmd 990 network-get-interfaces | grep -m3 '"ip-address"'
qm guest exec 990 -- cat /etc/os-release
qm stop 990 && qm destroy 990 --purge
```

---

## 6. Create the control-plane VM

Sizing (topology A):

| Mode | vCPU | RAM | Disk |
|---|---|---|---|
| demo | 2 | 4 GiB | 32 GiB |
| production | 4 | 8 GiB | 64 GiB + object storage (uploads, ISOs) |

Clone it from the Ubuntu template and give it a static address. Put your SSH
public key in `/root/nevatal-admin.pub` on the Proxmox host first.

```bash
qm clone 9000 150 --name nevatal-cp --full 1 --storage local-zfs
qm set 150 --cores 4 --memory 8192 --onboot 1
# Management network (not the customer VLAN the template may carry), with the
# Proxmox firewall enabled on the NIC (rules in section 12):
qm set 150 --net0 virtio,bridge=vmbr0,firewall=1
qm set 150 --ciuser nevatal --sshkeys /root/nevatal-admin.pub \
  --ipconfig0 ip=10.0.0.50/24,gw=10.0.0.1 --nameserver 1.1.1.1
qm disk resize 150 scsi0 64G
qm start 150
```

Then log in: `ssh nevatal@10.0.0.50`. The token test from section 4.3 should
now work from this VM (`sudo apt install -y curl` if needed).

---

## 7. Install Docker Engine

On the control-plane VM (Ubuntu 24.04), from Docker's apt repository:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git openssl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker "$USER"     # log out and back in afterwards

docker version
docker compose version              # must be v2.24 or newer
```

---

## 8. Get the code

```bash
sudo mkdir -p /opt/nevatal-cloud && sudo chown "$USER": /opt/nevatal-cloud
git clone https://github.com/thirza258/nevatal-cloud.git /opt/nevatal-cloud
cd /opt/nevatal-cloud
./deploy.sh --help
```

`deploy.sh` needs only Docker: it builds the images (Go and Node run inside
the build containers), validates `.env`, runs the migrations in a container and
waits for the API health check through nginx.

---

## 9. Demo deployment

```bash
cd /opt/nevatal-cloud
cp deploy/env/demo.env.example .env
chmod 600 .env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$(openssl rand -hex 32)/" .env
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 32)/" .env
nano .env     # DOMAIN, APP_BASE_URL; for HTTPS: COMPOSE_PROFILES=tls and HTTP_BIND=127.0.0.1
./deploy.sh --mode demo
```

What demo mode does:

- seeds the **public** accounts `admin@nevatal.id` / `AdminPass123!` and
  `demo@nevatal.id` / `DemoPass123!`, shows a role switcher and quick logins,
  and enables mock social login;
- forces the simulated hypervisor (`PROXMOX_MOCK=true`), whatever `.env` says;
  no Proxmox token belongs on a demo host;
- runs exactly one API replica.

> Anyone on the internet can log in as the demo administrator. Never put real
> data, a real Proxmox token, or a paid OpenRouter key without a spending cap
> on a demo host.

Reset the demo every night (deletes the demo database and uploaded files,
keeps the TLS certificate):

```bash
crontab -e
# add:
15 3 * * * cd /opt/nevatal-cloud && ./deploy.sh --reset-demo --yes --no-build >> "$HOME/nevatal-demo-reset.log" 2>&1
```

---

## 10. Production deployment

1. Finish sections 4 and 5 (token and templates).
2. Create `.env`:

```bash
cd /opt/nevatal-cloud
cp deploy/env/production.env.example .env
chmod 600 .env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$(openssl rand -hex 32)/" .env
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 32)/" .env
sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=$(openssl rand -hex 16)/" .env
grep '^ADMIN_PASSWORD=' .env       # write it down, then log in and change it
nano .env
```

3. In `.env` set at least:

```dotenv
DOMAIN=cloud.nevatal.id
APP_BASE_URL=https://cloud.nevatal.id
ADMIN_EMAIL=you@your-company.example
PROXMOX_MOCK=false
PROXMOX_HOST=10.0.0.2
PROXMOX_PORT=8006
PROXMOX_TOKEN_ID=nevatal@pve!control-plane
PROXMOX_TOKEN_SECRET=<value printed by "pveum user token add">
PROXMOX_NODE_NAME=pve-dell-r640
PROXMOX_STORAGE=local-zfs
PROXMOX_VERIFY_TLS=false        # see section 21 for true
# HTTPS with Caddy on this VM:
COMPOSE_PROFILES=tls
HTTP_BIND=127.0.0.1
```

4. Deploy and check:

```bash
./deploy.sh --mode production

curl -fsS http://127.0.0.1:3080/health
curl -fsS http://127.0.0.1:3080/api/v1/system/config
# expect: "environment":"production","demo_mode":false,"proxmox_mock":false
./deploy.sh --status
```

`deploy.sh` refuses to deploy while any `CHANGE_ME` is left, when `JWT_SECRET`
is shorter than 32 characters or a public default, without `ADMIN_EMAIL` /
`ADMIN_PASSWORD` (12+ characters), without the Proxmox token when
`PROXMOX_MOCK=false`, or with a well-known database password. The API applies
the same rules and will not start with an unsafe configuration.

Notes:

- `POSTGRES_PASSWORD` is applied only when the database volume is created.
  Changing it later also needs `ALTER ROLE ... PASSWORD` inside PostgreSQL.
- Avoid `$` in `.env` values: Docker Compose treats it as a variable.
- Demo and production use different `COMPOSE_PROJECT_NAME`s, so they never
  share a database volume on the same VM.

---

## 11. TLS / HTTPS options

The containers speak plain HTTP; terminate TLS in front of them and send HSTS
there.

**a) Caddy on the control-plane VM (compose profile `tls`).** Set
`COMPOSE_PROFILES=tls`, `DOMAIN`, `HTTP_BIND=127.0.0.1` in `.env`, forward
ports 80 and 443 (TCP and UDP) to the VM, run `./deploy.sh`. Caddy obtains and
renews the Let's Encrypt certificate (kept in the `caddy_data` volume) and
adds HSTS (`deploy/caddy/Caddyfile`).

**b) Cloudflare Tunnel (no inbound ports, no public IP).** Create a
remotely-managed tunnel in the Cloudflare dashboard, put its token in
`CLOUDFLARE_TUNNEL_TOKEN`, set `COMPOSE_PROFILES=tunnel` (plus `edge` for
customer subdomains) and keep `HTTP_BIND=127.0.0.1`. The `cloudflared`
container reaches the web UI as `http://frontend:8080` over the compose
network. Step-by-step setup, both tunnel styles and the Cloudflare limits
(100 MB request bodies on Free/Pro, which caps uploads; ~100 s per response):
section 20.

**c) An existing reverse proxy or HAProxy.** Point it at
`http://10.0.0.50:3080` (set `HTTP_BIND=10.0.0.50` and allow only the proxy in
the VM firewall). The proxy must send `X-Forwarded-Proto: https` and
`X-Forwarded-For`, allow large bodies (uploads up to `NGINX_CLIENT_MAX_BODY_SIZE`,
ISOs up to `NGINX_ISO_MAX_BODY_SIZE`), long timeouts and WebSocket upgrades.
`deploy/loadbalancer/haproxy/haproxy.cfg` is a complete example. If the proxy
is not in a private range, list its address in `TRUSTED_PROXIES`.

---

## 12. Firewall

Goal: the world reaches only 80/443 of the control-plane VM; the Proxmox API
(8006) and SSH reach the host only from the admin network and the
control-plane VM.

> Keep an iDRAC or *Node -> Shell* session open while you change firewall
> rules, and check with `pve-firewall compile` before enabling them.

**Control-plane VM** (`/etc/pve/firewall/150.fw` on the Proxmox host):

```ini
[OPTIONS]
enable: 1
policy_in: DROP

[RULES]
IN ACCEPT -p tcp -dport 80
IN ACCEPT -p tcp -dport 443
IN ACCEPT -p udp -dport 443
IN SSH(ACCEPT) -source 10.0.10.0/24
```

The NIC needs the firewall flag (set in section 6). For an existing VM, add it
while keeping the current MAC address:

```bash
qm config 150 | grep '^net0:'      # no "firewall=1" yet? then:
qm set 150 --net0 "$(qm config 150 | awk '/^net0:/ {print $2}'),firewall=1"
```

**Proxmox host** (`/etc/pve/firewall/cluster.fw`). With the datacenter
firewall enabled, the host accepts GUI/API (8006), SSH, VNC and SPICE only from
the `management` IP set plus the auto-detected `local_network`. On a single
host, pin `local_network` to the host itself so customer VMs on the same LAN do
not count as "local":

```ini
[OPTIONS]
enable: 1

[ALIASES]
local_network 10.0.0.2

[IPSET management]
10.0.10.0/24   # admin workstations
10.0.0.50      # Nevatal control-plane VM (uses the API token)
```

Add the k3s node IPs (topology C) or the app VMs (topology B) to `management`
as well, since they call the API.

**With a Cloudflare Tunnel** nothing inbound is needed: drop the 80/443 rules
from `150.fw`. A second NIC on the customer VLAN for the subdomain edge gets
its own rules (section 20.4).

---

## 13. Verify the Proxmox integration and the Docker viewer

On the Proxmox host (as root; this checks the host, not the token):

```bash
pvesh get /nodes/pve-dell-r640/status --output-format json | head -c 300; echo
qm guest cmd <vmid> ping                     # agent answers?
qm guest exec <vmid> -- docker ps            # Docker inside the guest
```

From the control-plane VM, with the token (this is what Nevatal does):

```bash
PVE=https://10.0.0.2:8006/api2/json
TOKEN='nevatal@pve!control-plane=<secret>'
VMID=101

curl -sk -H "Authorization: PVEAPIToken=$TOKEN" -H 'Content-Type: application/json' \
  -d '{"command":["docker","ps","--format","{{.Names}} {{.Status}}"]}' \
  "$PVE/nodes/pve-dell-r640/qemu/$VMID/agent/exec"
# -> {"data":{"pid":1234}}
curl -sk -H "Authorization: PVEAPIToken=$TOKEN" \
  "$PVE/nodes/pve-dell-r640/qemu/$VMID/agent/exec-status?pid=1234"
# -> {"data":{"exited":1,"exitcode":0,"out-data":"web Up 3 hours\n"}}
```

(Not run against a live Proxmox while writing this guide.)

**Docker viewer requirements** (customers see it on their VM page, admins on
any VM): `qemu-guest-agent` installed and running in the guest, the VM started
with `--agent enabled=1` (a running VM needs a full stop/start after enabling
it), Docker installed in the guest, and the token privilege from section 4
(`VM.Monitor` on PVE 8, `VM.GuestAgent.Unrestricted` on PVE 9). RHEL-family
guests block `guest-exec` in the agent's default configuration
(`/etc/sysconfig/qemu-ga`); Debian and Ubuntu do not.

---

## 14. Migrating from demo to production

- **Start production with a fresh database.** The production template uses a
  different `COMPOSE_PROJECT_NAME` (`nevatal-prod`) than the demo
  (`nevatal-demo`), so `./deploy.sh --mode production` creates new volumes.
  Stop the demo first (`docker compose -p nevatal-demo down`).
- Never reuse a demo database: it contains public accounts and simulated VMs
  whose VMIDs may collide with real VMs on your node.

If a database ever ran in demo mode and you must keep it:

1. Generate a **new `JWT_SECRET`** (invalidates every session issued during the
   demo).
2. Make sure `ADMIN_EMAIL` is your own address, not `admin@nevatal.id`.
3. Start in production mode. The API suspends demo accounts that still use the
   public passwords (log line `SECURITY: suspended ...`).
4. Check and lock them by hand as well:

```bash
cd /opt/nevatal-cloud
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
SELECT email, role, status FROM users WHERE email IN ('admin@nevatal.id', 'demo@nevatal.id');
UPDATE users
   SET status = 'suspended', role = 'customer', password_hash = NULL, updated_at = now()
 WHERE email IN ('admin@nevatal.id', 'demo@nevatal.id');
SQL
```

(The accounts are suspended rather than deleted: instances reference their
owner with `ON DELETE RESTRICT`.)

---

## 15. Backups and restore

**Whole VM (vzdump).** *Datacenter -> Backup -> Add*, or once from the shell:

```bash
vzdump 150 --mode snapshot --compress zstd --storage <backup-storage>
```

**Database, nightly** (on the control-plane VM):

```bash
mkdir -p ~/backups
crontab -e
# add:
30 2 * * * cd /opt/nevatal-cloud && docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$HOME/backups/db-$(date +\%F).dump" && find "$HOME/backups" -name 'db-*.dump' -mtime +14 -delete
```

**Object storage files** (`storagedata` volume; project `nevatal-prod`):

```bash
docker run --rm -v nevatal-prod_storagedata:/data:ro -v "$HOME/backups:/backup" alpine:3.24 \
  tar -C /data -czf "/backup/storage-$(date +%F).tgz" .
```

**Restore** into a running stack:

```bash
docker compose exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists' < ~/backups/db-2026-11-01.dump
docker run --rm -v nevatal-prod_storagedata:/data -v "$HOME/backups:/backup" alpine:3.24 \
  tar -C /data -xzf /backup/storage-2026-11-01.tgz
```

Copy backups off the R640 (another host, NAS or Proxmox Backup Server).

---

## 16. Upgrades and rollback

```bash
cd /opt/nevatal-cloud
git pull
./deploy.sh
```

- The API applies new database migrations at start-up (serialized with a
  PostgreSQL advisory lock); `deploy.sh` also runs them first in a one-off
  container so a failing migration stops the deploy before the API restarts.
- Docker Compose has no rolling update: all API replicas restart together, and
  provisioning jobs run inside the replica that accepted them. Check the admin
  job queue (*Admin -> Jobs*) before upgrading; a replica killed mid-job leaves
  that job failed (retry it from the admin UI).
- Take a database backup before upgrades that contain migrations.

**Rolling back a release**

```bash
./deploy.sh --rollback          # stops the API, reverts ONE migration (asks first)
git checkout <previous-tag>
./deploy.sh --skip-migrate
```

Run `--rollback` with the **newer** code still checked out: only it contains the
down-migration. Starting the newer release again re-applies the migration.

---

## 17. Load balancing

- **Inside one VM (topologies A and B).** The frontend nginx resolves the
  `backend` service name every 5 s and balances `/api` across all replicas with
  `least_conn`, upstream keepalive, WebSocket pass-through, and retries of
  idempotent requests on another replica (`proxy_next_upstream error timeout
  http_502 http_503 http_504`; POST/PATCH are never retried). Scale with
  `BACKEND_REPLICAS` in `.env` (production default 2) and `./deploy.sh`.
- All replicas on one host share the `storagedata` volume. Across hosts the
  object storage must be shared (NFS in topology B, a ReadWriteMany volume in
  topology C).
- **Demo stays at exactly one replica**: the simulated hypervisor lives in
  process memory.
- Known limitation: if a replica disappears abruptly (crash, `docker kill`), a
  request that is already waiting on it while nginx refreshes DNS can fail once
  with 502/504 instead of being retried. Graceful restarts (`SIGTERM`, 60 s
  grace) do not hit this.
- **Across VMs (topology B)**: HAProxy with `balance leastconn`, `/health`
  checks and a keepalived VIP (section 18). **In Kubernetes (topology C)**: the
  `backend` Service is headless, so nginx sees every ready pod directly;
  Traefik balances the frontends.

---

## 18. Topology B: several VMs behind HAProxy + keepalived

Files: `deploy/loadbalancer/haproxy/haproxy.cfg`,
`deploy/loadbalancer/keepalived/keepalived.conf`,
`deploy/compose/multi-vm.yml`.

**1. PostgreSQL VM (10.0.0.20).** Docker as in section 7, then:

```bash
docker volume create nevatal-pgdata
docker run -d --name nevatal-db --restart unless-stopped \
  -e POSTGRES_USER=nevatal -e POSTGRES_PASSWORD="$(openssl rand -hex 32 | tee ~/pg-password.txt)" \
  -e POSTGRES_DB=nevatal_cloud \
  -v nevatal-pgdata:/var/lib/postgresql/data \
  -p 10.0.0.20:5432:5432 postgres:16-alpine -c max_connections=200
```

Allow 5432 only from the app VMs (Proxmox VM firewall).

**2. NFS VM (10.0.0.21).**

```bash
sudo apt-get install -y nfs-kernel-server
sudo mkdir -p /srv/nevatal/storage && sudo chown 10001:10001 /srv/nevatal/storage
echo "/srv/nevatal/storage 10.0.0.11(rw,sync,no_subtree_check) 10.0.0.12(rw,sync,no_subtree_check)" | sudo tee -a /etc/exports
sudo exportfs -ra
```

**3. App VMs (10.0.0.11, 10.0.0.12).** Sections 6-8 on each (plus
`sudo apt-get install -y nfs-common`), then:

```bash
cd /opt/nevatal-cloud
cp deploy/env/production.env.example .env && chmod 600 .env
# Same JWT_SECRET, ADMIN_* and PROXMOX_* on every app VM. Then set:
#   COMPOSE_FILE=docker-compose.yml:deploy/compose/production.yml:deploy/compose/multi-vm.yml
#   EXTERNAL_DATABASE_URL=postgres://nevatal:<pg-password>@10.0.0.20:5432/nevatal_cloud?sslmode=disable
#   NFS_SERVER=10.0.0.21
#   NFS_EXPORT=/srv/nevatal/storage
#   HTTP_BIND=10.0.0.11            (this VM's address)
#   HTTP_PORT=3080
#   COMPOSE_PROFILES=              (no Caddy: HAProxy terminates TLS)
#   POSTGRES_PASSWORD=<any hex>    (unused here, but must not be CHANGE_ME)
./deploy.sh --mode production
```

Allow 3080 on the app VMs only from the two LB VMs.

**4. Load-balancer VMs (10.0.0.5, 10.0.0.6).**

```bash
sudo apt-get install -y haproxy keepalived certbot
sudo cp deploy/loadbalancer/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg   # edit server lines
sudo cp deploy/loadbalancer/keepalived/keepalived.conf /etc/keepalived/keepalived.conf   # lb2: swap marked values
sudo mkdir -p /etc/haproxy/certs
# Temporary self-signed certificate so HAProxy can start before Let's Encrypt:
sudo openssl req -x509 -newkey rsa:2048 -nodes -days 7 -subj "/CN=cloud.nevatal.id" \
  -keyout /tmp/k.pem -out /tmp/c.pem
sudo sh -c 'cat /tmp/c.pem /tmp/k.pem > /etc/haproxy/certs/nevatal.pem && chmod 600 /etc/haproxy/certs/nevatal.pem && rm /tmp/k.pem /tmp/c.pem'
sudo haproxy -c -f /etc/haproxy/haproxy.cfg && sudo systemctl restart haproxy
sudo keepalived -t -f /etc/keepalived/keepalived.conf && sudo systemctl restart keepalived
ip -br addr show ens18      # the VIP 10.0.0.10 appears on the active node
```

Let's Encrypt on the node that holds the VIP (HAProxy forwards
`/.well-known/acme-challenge/` to port 8888; outline, not verified here):

```bash
sudo certbot certonly --standalone --http-01-port 8888 -d cloud.nevatal.id
sudo tee /etc/letsencrypt/renewal-hooks/deploy/haproxy.sh >/dev/null <<'EOF'
#!/bin/sh
cat /etc/letsencrypt/live/cloud.nevatal.id/fullchain.pem /etc/letsencrypt/live/cloud.nevatal.id/privkey.pem > /etc/haproxy/certs/nevatal.pem
chmod 600 /etc/haproxy/certs/nevatal.pem
systemctl reload haproxy
# copy the PEM to the other LB as well (scp/rsync) and reload HAProxy there
EOF
sudo chmod 755 /etc/letsencrypt/renewal-hooks/deploy/haproxy.sh
sudo /etc/letsencrypt/renewal-hooks/deploy/haproxy.sh
```

Proxmox firewall for the LB VMs: allow 80/443 from anywhere and IP protocol 112
(VRRP) between `10.0.0.5` and `10.0.0.6` (e.g. `IN ACCEPT -source 10.0.0.6 -p vrrp`,
not verified here). If the NIC has the firewall *IP filter* option on, add the
VIP to the VM's `ipfilter-net0` IP set. HAProxy statistics:
`ssh -L 8404:127.0.0.1:8404 nevatal@10.0.0.5`, then open
`http://127.0.0.1:8404/stats`.

---

## 19. Topology C: k3s cluster

Files: `deploy/k8s/base`, `deploy/k8s/overlays/{demo,production}`,
`deploy/k8s/components/edge` (production only),
`deploy/k8s/addons/{metallb,cert-manager,traefik,cloudflared}`.

**1. VMs.** Three clones of template 9000 (4 vCPU, 8 GiB, 64 GiB each) at
10.0.0.31-33, as in section 6.

**2. k3s.** On the first VM (ServiceLB disabled because MetalLB provides the
LoadBalancer IP):

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable servicelb" sh -
sudo cat /var/lib/rancher/k3s/server/node-token
```

On the other two:

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://10.0.0.31:6443 K3S_TOKEN=<node-token> sh -
```

Use `sudo k3s kubectl ...` on the server, or copy `/etc/rancher/k3s/k3s.yaml`
to your workstation (replace `127.0.0.1` with `10.0.0.31`).

**3. Add-ons.**

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.16.0/config/manifests/metallb-native.yaml
kubectl -n metallb-system wait --for=condition=Available deploy/controller --timeout=180s
kubectl apply -k deploy/k8s/addons/metallb            # edit the address range first

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.yaml
kubectl -n cert-manager wait --for=condition=Available deploy --all --timeout=180s
kubectl apply -k deploy/k8s/addons/cert-manager       # set your e-mail first

kubectl apply -k deploy/k8s/addons/traefik            # Traefik: allow uploads longer than 60 s
kubectl -n kube-system get svc traefik                # EXTERNAL-IP from the MetalLB pool
```

Point DNS (or your router's port forward for 80/443) at Traefik's
`EXTERNAL-IP`.

**4. Shared storage for the production overlay.** Object storage must be
`ReadWriteMany`. With the NFS VM from topology B, the NFS subdir provisioner
can be installed through k3s' built-in Helm controller (install `nfs-common`
on every k3s VM first; not verified here):

```yaml
# nfs-provisioner.yaml -> kubectl apply -f nfs-provisioner.yaml
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: nfs-subdir-external-provisioner
  namespace: kube-system
spec:
  repo: https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/
  chart: nfs-subdir-external-provisioner
  targetNamespace: kube-system
  valuesContent: |-
    nfs:
      server: 10.0.0.21
      path: /srv/nevatal/k8s
    storageClass:
      name: nfs-client
```

It matches `storageClassName: nfs-client` in
`deploy/k8s/overlays/production/storage-rwx.yaml` (Longhorn: use `longhorn`).
Without RWX storage, delete that patch and pin the backend HPA to one replica.

**5. Images.** The R640 is amd64; build for it (also from an arm64 laptop):

```bash
TAG=1.2.0
docker buildx build --platform linux/amd64 --build-arg VERSION=$TAG \
  -t ghcr.io/thirza258/nevatal-cloud-backend:$TAG --push backend/
docker buildx build --platform linux/amd64 \
  -t ghcr.io/thirza258/nevatal-cloud-frontend:$TAG --push frontend/
```

Private registry: add an `imagePullSecret`. Without a registry, build with
`--load` instead of `--push`, then import into every k3s node:

```bash
docker save -o nevatal-images.tar ghcr.io/thirza258/nevatal-cloud-backend:$TAG ghcr.io/thirza258/nevatal-cloud-frontend:$TAG
scp nevatal-images.tar nevatal@10.0.0.31:   # repeat for .32 and .33
ssh nevatal@10.0.0.31 'sudo k3s ctr images import nevatal-images.tar'
```

Set `newTag` in the overlay's `kustomization.yaml` to `$TAG`. The
`ghcr.io/thirza258/...` images referenced there are not published: build and
push (or import) them first, or point `newName` at your own registry.
Otherwise the pods stay in `ImagePullBackOff`.

**6. Deploy.**

```bash
cd deploy/k8s/overlays/production          # or overlays/demo
cp secrets.env.example secrets.env && chmod 600 secrets.env
nano secrets.env                           # openssl rand -hex 32 for the secrets
nano kustomization.yaml                    # PROXMOX_HOST, hosts, image tags
kubectl kustomize . | less                 # review
kubectl apply -k .
kubectl -n nevatal-cloud rollout status deploy/backend
kubectl -n nevatal-cloud get pods,svc,ingress,hpa,pdb
curl -fsS https://cloud.nevatal.id/api/v1/system/config
```

What the manifests do: non-root pods with read-only root filesystems, startup /
readiness / liveness probes on `/health` (backend) and `/nginx-health`
(frontend), a 20 s `preStop` drain and 90 s grace period for the API,
PodDisruptionBudgets, NetworkPolicies (PostgreSQL only from the API, API only
from the frontend, frontend only from Traefik), HPAs in production (API 2-6,
frontend 2-4 at 70 % CPU, slow scale-down because jobs run in-process), and in
demo exactly one API pod with the `Recreate` strategy. The k3s nodes call the
Proxmox API: add their IPs to the `management` IP set (section 12).

**7. Optional: Cloudflare Tunnel and customer subdomains.** The production
overlay also runs the subdomain edge (2 pods); publish it and the control
plane through `deploy/k8s/addons/cloudflared` instead of Traefik. Set the
edge's network settings in the overlay first: section 20.8.

---

## 20. Cloudflare Tunnel and Railway-style subdomains (*.nevatal.id)

Customers publish one port of a VM at `https://<name>.nevatal.id` (like
Railway or Render): they pick a name, a VM and a port in the dashboard, and
the app is live a few seconds later with a valid certificate. The R640 needs no
public IP and no port forwarding: Cloudflare terminates HTTPS and a Cloudflare
Tunnel carries the traffic to `cloudflared`, which runs next to Nevatal and
only makes outbound connections.

This is a **production-only** feature. Demo mode always turns it off (demo
accounts are public and simulated VM addresses may belong to real hosts), and
the `edge` proxy refuses to start with `DEMO_MODE=true`. The demo itself can
still be published through a tunnel (`demo.nevatal.id`, section 20.3).

What you need:

- the domain's zone on Cloudflare (Free plan is enough);
- a Cloudflare Tunnel (section 20.3);
- the `edge` proxy on a host that can reach the customer VMs (section 20.4);
- `PUBLIC_DOMAIN_ROOT`, `EDGE_TARGET_CIDRS` and `EDGE_DENY_IPS` (sections 20.2,
  20.5, 20.7).

### 20.1 How a request flows

```
https://myapp.nevatal.id
   |  DNS: *.nevatal.id  CNAME  <TUNNEL_UUID>.cfargotunnel.com  (proxied)
   v
Cloudflare edge: TLS (Universal SSL), WAF, DDoS protection
   |  Cloudflare Tunnel (opened from inside by cloudflared)
   v
cloudflared ingress rules, top to bottom, first match wins:
   1. cloud.nevatal.id  -> http://frontend:8080   control plane (nginx -> API)
   2. demo.nevatal.id   -> http://<demo VM>:3080  optional
   3. *.nevatal.id      -> http://edge:8088       customer apps
   4. (anything else)   -> http_status:404
   v
nevatal-edge: looks up "myapp" in its routing table (reloaded from
PostgreSQL every 2 s) and proxies to the VM's private IP:port
   v
customer VM 10.20.0.23:3000
```

The edge proxies a name only when the domain is enabled, the VM is running and
has an address, the address is inside `EDGE_TARGET_CIDRS`, is not in
`EDGE_DENY_IPS` (the Proxmox host from `PROXMOX_HOST` is always denied), is not
loopback or link-local, and no other instance reports the same address. The
check runs again when the edge opens the connection. Otherwise the visitor gets
an error page: 404 (unknown or disabled name), 503 (VM stopped) or 502 (address
not routable, address conflict, app not answering).

The edge rebuilds the forwarding headers from what Cloudflare established
(`X-Forwarded-For`, `X-Real-Ip`, `Cf-Connecting-Ip`, `X-Forwarded-Proto`,
`X-Forwarded-Host`) and drops any the visitor sent. WebSockets, streaming
responses and long uploads pass through (no overall request timeout at the
edge).

> **Ingress order matters.** Checked with `cloudflared tunnel ingress rule`
> (cloudflared 2026.10.7): with `*.nevatal.id` listed before
> `cloud.nevatal.id`, a request for `https://cloud.nevatal.id/` matches the
> wildcard and lands on the edge (which answers 404), so the control plane is
> unreachable. Keep the wildcard after every explicit hostname.

### 20.2 Choose the tenant domain (security)

With `PUBLIC_DOMAIN_ROOT=nevatal.id`, customer apps (`myapp.nevatal.id`) and
the control plane (`cloud.nevatal.id`) share one registrable domain, so
browsers treat them as the **same site**:

- **Reputation.** Anyone can publish anything on a free subdomain. One
  phishing or malware report against a tenant app can get the whole
  `nevatal.id` blocklisted (browser safe-browsing lists, corporate web
  filters, mail reputation): the control plane and every other customer go
  down with it.
- **Cookies.** A cookie set with `Domain=nevatal.id` (by the control plane, a
  marketing site or any tenant app) is sent to every `*.nevatal.id` host,
  including apps of other customers, and tenant apps can set such cookies
  themselves ("cookie tossing"). `SameSite` does not help between same-site
  hosts.

**Recommendation: use a separate registrable domain for tenant apps**, the way
Railway uses `up.railway.app` and GitHub uses `github.io`. Register e.g.
`nevatal.app`, add it to the same Cloudflare account and set
`PUBLIC_DOMAIN_ROOT=nevatal.app`; the control plane stays at
`cloud.nevatal.id`. The tunnel can carry hostnames of both zones. Later you can
also ask for the tenant domain to be added to the Public Suffix List, so
browsers treat every `<name>.nevatal.app` as its own site (a manual review
that takes weeks; not done here).

The control plane is built to stay safe either way: its session cookie is
`__Host-nevatal_token` (Secure, `Path=/`, no `Domain`, so sibling hosts cannot
set or overwrite it), the cookie is accepted only on GET/HEAD, and every
state-changing API call needs an `Authorization: Bearer` token, so a tenant app
cannot make the browser act on the API (CSRF). `./deploy.sh` warns when the
control plane lives inside `PUBLIC_DOMAIN_ROOT`.

The examples below use `nevatal.id`, as in the rest of this guide; replace it
with your tenant domain.

### 20.3 One-time Cloudflare setup

In the Cloudflare dashboard of the zone, also turn on *SSL/TLS -> Edge
Certificates -> Always Use HTTPS* (tenant apps then never answer on plain
HTTP) and keep *Network -> WebSockets* on (the default).

**a) Remotely-managed tunnel (recommended).** Used by the compose profile
`tunnel` and by `deploy/k8s/addons/cloudflared`; the ingress rules live in the
dashboard.

1. *Zero Trust -> Networks -> Tunnels -> Create a tunnel -> Cloudflared*, name
   it (e.g. `nevatal-prod`), save. The next page shows install commands that
   end in `--token eyJ...`. Copy **only the token** into `.env`
   (`CLOUDFLARE_TUNNEL_TOKEN=eyJ...`) or `tunnel.env` (Kubernetes). Do not
   run the install command: the container is the connector.
2. On the tunnel's *Public hostnames* tab (newer dashboards may call it
   *Published application routes*; the labels change over time and were not
   checked here), add in this order:

| # | Subdomain | Domain | Service (compose) | Service (Kubernetes) |
|---|---|---|---|---|
| 1 | `cloud` | `nevatal.id` | `HTTP` `frontend:8080` | `HTTP` `frontend.nevatal-cloud.svc.cluster.local:80` |
| 2 | `demo` (optional) | `nevatal.id` | `HTTP` `10.0.0.60:3080` (demo VM) | `HTTP` `frontend.nevatal-cloud.svc.cluster.local:80` (demo cluster) |
| 3 | `*` | `nevatal.id` (or `nevatal.app`) | `HTTP` `edge:8088` | `HTTP` `edge.nevatal-cloud.svc.cluster.local:8088` |

   The final catch-all (`http_status:404`) is implicit. Cloudflare creates
   the proxied `CNAME` records for `cloud` and `demo` itself. Whether the
   dashboard accepts `*` as a public hostname and creates the wildcard DNS
   record is **not verified here**. If it does not, or the record is missing
   afterwards, create it by hand under *DNS -> Records -> Add record*:

   | Type | Name | Target | Proxy status |
   |---|---|---|---|
   | `CNAME` | `*` | `<TUNNEL_UUID>.cfargotunnel.com` | Proxied (orange cloud) |

   `<TUNNEL_UUID>` is the tunnel ID on the tunnel's overview page. If the
   wildcard was added before `cloud`, move it to the end of the list.

**b) Locally-managed tunnel (config file).** For a connector you run yourself,
e.g. in a small LXC (section 20.6). Install `cloudflared` from Cloudflare's
package repository first (follow their current instructions). Then, on that
machine (`tunnel login`, `create` and `service install` not verified here):

```bash
cloudflared tunnel login                   # opens a browser, writes ~/.cloudflared/cert.pem
cloudflared tunnel create nevatal-prod     # prints <TUNNEL_UUID>, writes ~/.cloudflared/<TUNNEL_UUID>.json
cloudflared tunnel route dns nevatal-prod cloud.nevatal.id
cloudflared tunnel route dns nevatal-prod demo.nevatal.id      # optional
cloudflared tunnel route dns nevatal-prod '*.nevatal.id'       # wildcard: not verified; else add the CNAME by hand (table above)

sudo install -d -m 0755 /etc/cloudflared
sudo cp ~/.cloudflared/<TUNNEL_UUID>.json /etc/cloudflared/
sudo cp /opt/nevatal-cloud/deploy/cloudflared/config.yml.example /etc/cloudflared/config.yml
sudo nano /etc/cloudflared/config.yml      # <TUNNEL_UUID>, addresses, tenant domain
sudo cloudflared --config /etc/cloudflared/config.yml tunnel ingress validate
sudo cloudflared --config /etc/cloudflared/config.yml tunnel ingress rule https://cloud.nevatal.id
sudo cloudflared service install           # systemd unit using /etc/cloudflared/config.yml
systemctl status cloudflared
```

`deploy/cloudflared/config.yml.example` holds the ordered ingress rules from
section 20.1 (`ingress validate` and `ingress rule` were run against it with
cloudflared 2026.10.7). Keep the `.json` credentials file private: it lets
anyone run your tunnel.

**c) DNS and certificate rules.**

- **First-level names only.** Universal SSL covers `nevatal.id` and
  `*.nevatal.id`, not `*.apps.nevatal.id`. A second level (`myapp.apps.nevatal.id`)
  would need Advanced Certificate Manager (paid). Nevatal hands out
  first-level names; use a separate domain (section 20.2) rather than a
  sub-zone.
- **Explicit DNS records win over the wildcard.** A name that has any record
  of its own (`cloud`, `www`, `vpn`, `mail`, ...) never matches
  `*.nevatal.id`, so a customer who claimed that name would never receive
  traffic. The same applies to a name that only exists because of a deeper
  record (e.g. `_acme-challenge.shop` makes `shop` exist). Add every such name
  to `RESERVED_SUBDOMAINS`. The built-in list already contains about 80 names
  (`www`, `cloud`, `api`, `app`, `admin`, `demo`, `docs`, `status`, `mail`,
  `auth`, `login`, `billing`, `edge`, `tunnel`, `vpn`, `proxmox`, ...), and the
  control plane's own label from `APP_BASE_URL` is always reserved.
- Deeper names such as `a.b.nevatal.id` still resolve through the wildcard
  but have no certificate (browser TLS error); the edge only serves
  `<label>.<PUBLIC_DOMAIN_ROOT>`.

### 20.4 Network: give the edge a path to the customer VMs

The edge connects to the VM's private address itself, so the host that runs it
(the control-plane VM in topology A) needs layer-3 access to the customer
network. Keep customer VMs on their own VLAN or bridge (template `net0` with
`tag=20`, section 5.3). On a flat LAN shared with infrastructure,
`EDGE_DENY_IPS` would have to list every Proxmox host, gateway, NAS and admin
UI, which is fragile.

**Option 1 (recommended): a second NIC on the customer VLAN.**

```bash
# On the Proxmox host: NIC 2 of VM 150 on VLAN 20, Proxmox firewall enabled.
qm set 150 --net1 virtio,bridge=vmbr0,tag=20,firewall=1
# Address on the customer network, NO gateway (the default route stays on net0).
qm set 150 --ipconfig1 ip=10.20.0.5/24
qm reboot 150      # cloud-init applies the new network config at boot (not verified here)
```

Or configure it inside the VM (the second virtio NIC is usually `ens19`):

```yaml
# /etc/netplan/60-customer.yaml, then: sudo netplan apply
network:
  version: 2
  ethernets:
    ens19:
      addresses: [10.20.0.5/24]
```

Check from the VM: `ping -c1 10.20.0.1` and `curl -m 5 http://<a customer VM>:<port>/`.

**Option 2: a route.** If a router/firewall (OPNsense, your gateway) already
routes between the management network and the customer network, allow new
connections only from `10.0.0.50` to `10.20.0.0/24` and nothing in the other
direction.

**Firewall the new NIC.** Customer VMs must never open connections to the
control plane. With a tunnel, `/etc/pve/firewall/150.fw` needs no 80/443 rules
at all (`-i` takes the VM's NIC key, `net0`/`net1`; run
`pve-firewall compile` before enabling, as in section 12):

```ini
[OPTIONS]
enable: 1
policy_in: DROP

[RULES]
# Customer VLAN (net1): nothing may open a connection to the control plane.
# Replies to the edge's own connections pass (the firewall is stateful).
IN DROP -i net1
# Management (net0): SSH from the admin workstations only.
IN SSH(ACCEPT) -i net0 -source 10.0.10.0/24
```

Docker turns on IP forwarding but sets the `FORWARD` policy to `DROP`, so the
VM does not route between `net0` and `net1`; the rule above also drops such
packets before they reach the VM.

Also:

- Put the control-plane VM's own customer address and the gateway into
  `EDGE_DENY_IPS` (`10.20.0.1,10.20.0.5`), or a customer could point a name at
  services listening on them.
- Customer VMs see the edge's connections coming from `10.20.0.5` (Docker
  masquerades the container). Customers allow their app port from that address.
- Docker's default address pools (`172.17.0.0/16` and up, `192.168.x.0/20`)
  must not overlap the customer network, or the containers cannot reach it.
  Change `default-address-pools` in `/etc/docker/daemon.json` if they do.
- Topology B: run the `edge` profile only on app VMs with a customer NIC.
  Several VMs may each run `cloudflared` with the same token.
- k3s: every node that may run an edge pod needs the NIC or route (or pin the
  edge Deployment with a `nodeSelector`). Pods leave through the node's
  address: add the nodes' customer addresses to `EDGE_DENY_IPS` and to the
  NetworkPolicy `except` list (section 20.8).

### 20.5 Anti-spoofing: why EDGE_TARGET_CIDRS and EDGE_DENY_IPS are mandatory

Nevatal learns a VM's address from the QEMU guest agent **inside the VM**,
that is from software the customer controls as root. A customer can make the
agent report any address: the Proxmox host, the gateway, the control plane or
another customer's VM. Without limits, `evil.nevatal.id` could then expose the
Proxmox API or a router's admin page to the internet.

What Nevatal enforces:

- the edge only connects to addresses inside `EDGE_TARGET_CIDRS` (required in
  production when `PUBLIC_DOMAIN_ROOT` is set: the API, the edge and
  `./deploy.sh` refuse to start without it), never to `EDGE_DENY_IPS` or the
  `PROXMOX_HOST` address, never to loopback, link-local or multicast
  addresses, and checks this again when it dials;
- an address reported by more than one instance is not routed at all
  (status `ip_conflict`), so a VM cannot take over another customer's names by
  copying its address. The worker prefers guest addresses inside
  `EDGE_TARGET_CIDRS` when a VM reports several.

What these rules cannot stop: inside the customer subnet, a VM can still
report an address that no other Nevatal instance uses (a machine on that VLAN
that Nevatal does not manage, a just-released address) and get its name
proxied there, or take another customer's names offline by reporting the same
address. **The real fix is to make each VM's address fixed and unforgeable:**

1. Give every VM a fixed address: a DHCP reservation for its MAC address
   (Nevatal clones use `ipconfig0=ip=dhcp`, section 5) or a static
   `ipconfig0`.
2. Turn on the Proxmox firewall **IP filter** for its NIC, so the VM cannot
   send packets (or ARP) from any other address:

```bash
# On the Proxmox host. Commands not verified here: try them on one test VM first.
NODE=pve-dell-r640; VMID=123; IP=10.20.0.23
qm config $VMID | grep '^net0:'      # no "firewall=1" yet? then (keeps the MAC):
qm set $VMID --net0 "$(qm config $VMID | awk '/^net0:/ {print $2}'),firewall=1"
pvesh create /nodes/$NODE/qemu/$VMID/firewall/ipset --name ipfilter-net0
pvesh create /nodes/$NODE/qemu/$VMID/firewall/ipset/ipfilter-net0 --cidr $IP
pvesh set /nodes/$NODE/qemu/$VMID/firewall/options \
  --enable 1 --ipfilter 1 --macfilter 1 --dhcp 1 --policy_in ACCEPT --policy_out ACCEPT
pve-firewall compile >/dev/null && echo "rules compile"
```

The same as a file, `/etc/pve/firewall/123.fw`:

```ini
[OPTIONS]
enable: 1
ipfilter: 1
macfilter: 1
dhcp: 1
policy_in: ACCEPT
policy_out: ACCEPT

[IPSET ipfilter-net0]
10.20.0.23
```

Notes (not verified here): VM firewalls only take effect while the datacenter
firewall is enabled (section 12); an enabled VM firewall drops incoming
traffic by default, hence `policy_in: ACCEPT` (customers keep managing their
own firewall); check how DHCP renewals and IPv6 link-local traffic behave
with the IP filter on your Proxmox version before rolling it out. Turning the
template's `net0` into `firewall=1` (section 5.3) makes every new clone
inherit the flag.

3. Watch the admin *Domains* page: it lists each domain's target address and
   live status, and `ip_conflict` / `ip_not_routable` point at spoofing or a
   misconfigured guest.

### 20.6 Where to run cloudflared

| Where | When | Service URLs for the ingress rules |
|---|---|---|
| Compose profile `tunnel` on the control-plane VM (recommended) | Topology A: one stack, nothing else to maintain | `http://frontend:8080`, `http://edge:8088` |
| A small LXC on the R640 | The tunnel should not depend on the Docker stack, it serves several VMs/stacks, or you prefer a locally-managed config | `http://10.0.0.50:3080`, `http://10.0.0.50:8088` |
| `deploy/k8s/addons/cloudflared` | Topology C | `http://frontend.nevatal-cloud.svc.cluster.local:80`, `http://edge.nevatal-cloud.svc.cluster.local:8088` |

**LXC variant.** Create an unprivileged Debian container with 1 vCPU and
256 MiB (e.g. CT `160`, `10.0.0.60`, on the management network), install
`cloudflared`, and either run `sudo cloudflared service install <TOKEN>`
(remotely-managed, not verified here) or use the config file from section
20.3 b. On the control-plane VM:

- `.env`: `HTTP_BIND=10.0.0.50`, `COMPOSE_PROFILES=edge` (no `tunnel`), and
  append `:deploy/compose/edge-publish.yml` to `COMPOSE_FILE` with
  `EDGE_BIND=10.0.0.50` and `EDGE_PORT_HOST=8088`;
- firewall: allow both ports only from the connector (the edge trusts the
  `Cf-Connecting-Ip` and `Cf-Visitor` headers, so nothing else may reach it):

```ini
IN ACCEPT -i net0 -source 10.0.0.60 -p tcp -dport 3080
IN ACCEPT -i net0 -source 10.0.0.60 -p tcp -dport 8088
```

**High availability.** Every connector started with the same token joins the
same tunnel, and Cloudflare spreads requests over them: e.g. the compose
`tunnel` profile plus an LXC, two app VMs in topology B, or the two replicas
of the Kubernetes add-on.

### 20.7 Configure and deploy (Docker Compose)

In `.env` of the production stack:

```dotenv
COMPOSE_PROFILES=edge,tunnel
HTTP_BIND=127.0.0.1
CLOUDFLARE_TUNNEL_TOKEN=eyJhIjoi...                # section 20.3 a
PUBLIC_DOMAIN_ROOT=nevatal.app                    # or nevatal.id (same-site, section 20.2)
RESERVED_SUBDOMAINS=                              # every other name with its own DNS record
MAX_DOMAINS_PER_USER=5
DOMAIN_QUARANTINE_DAYS=14                         # released names stay blocked for others
EDGE_TARGET_CIDRS=10.20.0.0/24                    # customer VM subnet(s); a single IP is allowed
EDGE_DENY_IPS=10.20.0.1,10.20.0.5                 # gateway, this VM; PROXMOX_HOST is added automatically
```

```bash
./deploy.sh --mode production
```

`./deploy.sh` refuses to deploy when `PUBLIC_DOMAIN_ROOT` is set without
`EDGE_TARGET_CIDRS`, the `tunnel` profile has no token, the `edge` profile
has no `PUBLIC_DOMAIN_ROOT` (the edge would not start), the demo enables
`edge`, or a CIDR/IP is malformed (`0.0.0.0/0` included). It warns about a
same-site tenant domain, CIDRs broader than `/16`, an empty deny list,
`PUBLIC_DOMAIN_ROOT` without the `edge` profile, and an edge that nothing can
reach (neither `tunnel` nor `edge-publish.yml`). It then starts `edge`
(health check `GET /__edge/health`) and `cloudflared` (health check
`cloudflared tunnel ready`, i.e. connected to Cloudflare) and waits for both.
Removing a profile later stops and removes that container on the next deploy
(`docker compose up --remove-orphans` alone would leave it running).

Verify:

```bash
./deploy.sh --status                 # edge health, tunnel readiness, subdomain settings
docker compose exec -T edge wget -qO- http://127.0.0.1:8088/__edge/health
# {"last_refresh":"...","live":1,"routes":1,"status":"ok"}
docker compose exec -T edge wget -qO- --header 'Host: myapp.nevatal.app' http://127.0.0.1:8088/
curl -fsS http://127.0.0.1:20241/ready          # cloudflared, host loopback (CLOUDFLARED_METRICS_PORT)
curl -sI https://myapp.nevatal.app               # from anywhere, once the name is claimed
```

`/__edge/health` answers 200 once the routing table is loaded (`"status":"stale"`
when the database has not answered for 30 s; the edge keeps serving the last
table) and 503 before the first load. The edge never migrates the database:
the API and `./deploy.sh` do.

Upgrades: `./deploy.sh` recreates the edge with the new image, so tenant apps
pause for a few seconds (compose has no rolling update). `cloudflared` is
pinned (`CLOUDFLARED_VERSION`, default `2026.10.7`, same tag in the k8s
add-on); update it deliberately every few months.

### 20.8 Kubernetes (k3s)

- `deploy/k8s/overlays/production` includes `deploy/k8s/components/edge`:
  an `edge` Deployment (2 replicas, same image, command `/app/nevatal-edge`,
  only the settings it needs: no JWT secret or Proxmox token), a ClusterIP
  Service `edge:8088`, a PodDisruptionBudget and NetworkPolicies (only
  cloudflared may reach the edge; the edge may reach DNS, PostgreSQL and the
  customer CIDR only). Set `PUBLIC_DOMAIN_ROOT`, `RESERVED_SUBDOMAINS`,
  `EDGE_TARGET_CIDRS`, `EDGE_DENY_IPS` in the overlay's `configMapGenerator`
  and the same customer CIDR in its `edge-egress` patch (`except` = the deny
  addresses inside that subnet; Kubernetes rejects `except` entries outside
  the `cidr`).
- `deploy/k8s/addons/cloudflared`: two connector replicas of a
  remotely-managed tunnel, token from a gitignored `tunnel.env`:

```bash
kubectl apply -k deploy/k8s/overlays/production          # first: creates the namespace
cd deploy/k8s/addons/cloudflared
cp tunnel.env.example tunnel.env && chmod 600 tunnel.env && nano tunnel.env
kubectl apply -k .
kubectl -n nevatal-cloud rollout status deploy/cloudflared
kubectl -n nevatal-cloud logs deploy/cloudflared | grep -i 'registered tunnel connection'
kubectl -n nevatal-cloud get pods -l app.kubernetes.io/component=edge
```

- Dashboard service URLs: `http://frontend.nevatal-cloud.svc.cluster.local:80`
  and `http://edge.nevatal-cloud.svc.cluster.local:8088` (section 20.3 a).
- There is **no Traefik Ingress for `*.nevatal.id`**: the tunnel replaces it.
  With all public traffic on the tunnel, MetalLB, cert-manager and Traefik are
  only needed for LAN access. The base Ingress for `cloud.nevatal.id` asks
  cert-manager for a Let's Encrypt certificate over HTTP-01, which fails
  without inbound port 80: remove its `cert-manager.io/cluster-issuer`
  annotation (or the Ingress) in that case.
- The demo overlay has no edge; the add-on can still publish
  `demo.nevatal.id` -> `frontend`.

### 20.9 Cloudflare limits that affect customer apps

- **Request body: 100 MB** on the Free and Pro plans (200 MB Business,
  500 MB Enterprise). Larger uploads get `413` from Cloudflare before reaching
  the app: use chunked or resumable uploads. This also caps uploads to the
  control plane through the tunnel; upload big ISOs from the LAN (SSH tunnel
  to `127.0.0.1:3080`) or directly to Proxmox.
- **Response time: about 100 seconds.** A proxied request fails with `524`
  when the origin has not started answering within ~100 s. Long jobs should
  answer at once (`202 Accepted`) and let the client poll, or stream (SSE,
  chunked responses).
- **WebSockets** work (enabled by default under *Network*). Cloudflare closes
  long-lived connections now and then (e.g. during its own deployments):
  clients must reconnect.
- **Caching.** Cloudflare caches static file types (images, CSS, JS, ...)
  according to `Cache-Control`; apps that serve per-user content under such
  extensions must send `Cache-Control: private` or `no-store`.
- **HTTP(S) only** at `<name>.nevatal.id`, on ports 80/443. SSH, databases and
  other TCP services are not published this way.

### 20.10 For customers: publishing an app

1. Make the app listen on `0.0.0.0:<PORT>` inside the VM (not `127.0.0.1`),
   speaking plain HTTP; HTTPS is Cloudflare's job. Examples:
   `python3 -m http.server 8000 --bind 0.0.0.0`,
   `app.listen(3000, '0.0.0.0')`, `docker run -p 3000:3000 ...`.
2. Allow the port from the platform's edge address (`10.20.0.5` in this
   guide), e.g. `sudo ufw allow from 10.20.0.5 to any port 3000 proto tcp`.
3. Keep the VM running with `qemu-guest-agent` (Nevatal reads the address
   through it).
4. In the dashboard open **Domains** (or the VM's page), choose a free name,
   the VM and the port. The name is checked as you type (reserved and taken
   names are refused).
5. The domain is live within a few seconds: `https://<name>.nevatal.id`.
   Its status shows why it is not: VM stopped, no address yet, address not
   routable, address conflict. A `502` page with status *live* means the app
   is not answering on that port.
6. The visitor's address arrives in `X-Forwarded-For`, `X-Real-Ip` and
   `Cf-Connecting-Ip`, the scheme in `X-Forwarded-Proto: https`.
7. Each account holds up to `MAX_DOMAINS_PER_USER` names; a released name is
   blocked for other accounts for `DOMAIN_QUARANTINE_DAYS` days.

---

## 21. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| VM stays without an IP; logs show `QEMU guest agent is not running` | Agent missing in the template, agent option off, or VM not restarted after enabling it | Install `qemu-guest-agent` in the image (section 5.2), `qm set <id> --agent enabled=1`, stop/start the VM, test `qm guest cmd <id> ping`. Check DHCP on the bridge. |
| `403 Permission check failed (/vms/123, VM.GuestAgent.Unrestricted)` (or `VM.Monitor`, `SDN.Use`, ...) | Role lacks the privilege, the ACL is missing for the user **or** the token (privsep = intersection), or a `NoAccess` ACL applies | Section 4; compare `pveum user token permissions nevatal@pve control-plane` with the table. |
| `401 authentication failure` | Wrong token id/secret format | `PROXMOX_TOKEN_ID=user@realm!tokenid`, `PROXMOX_TOKEN_SECRET=<uuid>`; recreate the token if the secret is lost. |
| `595` errors (`no route to host`, `Connection refused`, `Connection timed out`) | Control plane cannot reach `PROXMOX_HOST:8006`, or Proxmox cannot reach another cluster node | Check routing and the `management` IP set (section 12), `curl -k https://10.0.0.2:8006/api2/json/version` from the VM. |
| `x509: certificate signed by unknown authority` / `certificate is valid for ..., not 10.0.0.2` | `PROXMOX_VERIFY_TLS=true` with the self-signed Proxmox certificate | Keep `false` on an isolated management network, or give Proxmox a trusted certificate (*Datacenter -> ACME*) and set `PROXMOX_HOST` to its DNS name. |
| `413 Request Entity Too Large` on uploads | Body limit in nginx or in a proxy in front | Raise `NGINX_CLIENT_MAX_BODY_SIZE` / `NGINX_ISO_MAX_BODY_SIZE` in `.env` and redeploy; check Cloudflare / external proxy limits. |
| Uploads fail after ~60 s on k3s | Traefik v3 default `readTimeout` (60 s) | `kubectl apply -k deploy/k8s/addons/traefik`. |
| No real VMs are created; `/api/v1/system/config` shows `"proxmox_mock":true` | Demo mode (always mock), `PROXMOX_MOCK=true`, or incomplete Proxmox settings in development | Production: `PROXMOX_MOCK=false` plus host/token, redeploy; `./deploy.sh --status` prints the runtime mode. |
| API restarts with `unsafe production configuration` | Weak or missing `JWT_SECRET`, `ADMIN_*`, Proxmox settings | Fix the listed variables (`./deploy.sh` reports them before deploying). |
| `deploy.sh`: `Another Nevatal stack from this directory is running` | Demo and production both started from one checkout | `docker compose -p <old-project> down`, then deploy again. |
| `toomanyrequests: You have reached your unauthenticated pull rate limit` | Docker Hub anonymous pull limit | `docker login`, or wait and retry. |
| Docker viewer: empty list or `docker: not found` | Docker not installed in the guest, or RHEL guest blocks `guest-exec` | Install Docker (or use the Docker template), allow `guest-exec` in `/etc/sysconfig/qemu-ga`. |
| `502 Bad Gateway` from nginx | No healthy API replica | `docker compose ps`, `docker compose logs backend`. |
| `cloudflared` stays unhealthy; log `Register tunnel error from server side error="Failed to get tunnel"` | Wrong token (or a token of a deleted tunnel). The connector keeps retrying, so `./deploy.sh` waits for its health check (up to 300 s) and then stops with an error while the rest of the stack is already running | Copy the token again from the tunnel page (only the `eyJ...` value) into `CLOUDFLARE_TUNNEL_TOKEN`, `./deploy.sh`. The same wait happens when Cloudflare is unreachable at deploy time. |
| `cloudflared` never connects (repeated connection errors or timeouts towards the Cloudflare edge in `docker compose logs cloudflared`) | Outbound TCP/UDP 7844 blocked by a firewall upstream | Allow outbound 7844 (QUIC/UDP, or TCP for the HTTP/2 fallback) from the VM. |
| `Bind for 127.0.0.1:20241 failed: port is already allocated` | Another cloudflared (e.g. installed on the VM itself) uses the default metrics port | Stop the old connector, or set `CLOUDFLARED_METRICS_PORT`. |
| `https://cloud.nevatal.id` shows the edge's "not found" page | The `*` public hostname is listed before `cloud` | Move the wildcard to the end of the ingress rules (section 20.1). |
| Cloudflare `502 Bad gateway` page for `cloud.nevatal.id` | Wrong service URL in the dashboard (e.g. `localhost:3080` from inside the container) | Compose: `http://frontend:8080`; LXC: `http://10.0.0.50:3080` with `HTTP_BIND=10.0.0.50`; k8s: `http://frontend.nevatal-cloud.svc.cluster.local:80`. |
| `DNS_PROBE_FINISHED_NXDOMAIN` for `<name>.nevatal.id` | Wildcard DNS record missing | Add `CNAME * <TUNNEL_UUID>.cfargotunnel.com`, proxied (section 20.3). |
| A claimed name shows something else (control plane, an old server) | That name has its own DNS record, which wins over the wildcard | Delete the record, or add the name to `RESERVED_SUBDOMAINS` so it cannot be claimed. |
| TLS error on `a.b.nevatal.id` / `x.apps.nevatal.id` | Universal SSL covers first-level names only | Use first-level names (or Advanced Certificate Manager). |
| Edge page `502`, domain status `ip_not_routable` | VM address outside `EDGE_TARGET_CIDRS`, or listed in `EDGE_DENY_IPS` | Put customer VMs on the customer network, fix `EDGE_TARGET_CIDRS`; check which address the guest agent reports (`qm guest cmd <id> network-get-interfaces`). |
| Edge page `502`, status `live` | App listens on `127.0.0.1`, wrong port, or the guest firewall blocks the edge | From the control-plane VM: `curl -m 5 http://<vm-ip>:<port>/`; app on `0.0.0.0`, allow the port from the edge address (section 20.10). |
| Domain status `ip_conflict` | Two instances report the same address (duplicate DHCP/static address, or a spoofing guest) | Fix the addresses; pin them with DHCP reservations + Proxmox IP filter (section 20.5). |
| `edge` restarts with `Refusing to start: unsafe edge configuration` | `DEMO_MODE=true`, empty `PUBLIC_DOMAIN_ROOT`, or no `EDGE_TARGET_CIDRS` in production | Fix the listed variables; the edge never runs in demo mode. |
| `413` from Cloudflare on uploads / `524` on slow requests | Cloudflare limits: 100 MB request body (Free/Pro), ~100 s until the origin answers | Chunked uploads; answer long jobs at once and poll or stream (section 20.9). |
