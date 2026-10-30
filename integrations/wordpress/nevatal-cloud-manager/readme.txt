=== Nevatal Cloud VPS Manager & Orchestrator ===
Contributors: nevatal
Tags: vps, proxmox, cloud, woocommerce, hosting, servers, devops
Requires at least: 5.8
Tested up to: 6.7
Requires PHP: 7.4
License: GPLv2 or later

Connect WordPress and WooCommerce directly to your Nevatal Cloud Proxmox VE control plane.

== Description ==

Nevatal Cloud VPS Manager connects your WordPress site or WooCommerce digital store directly to your self-hosted Proxmox VE virtualization cluster.

Key Features:
- **WooCommerce Automatic VPS Provisioning:** When customers purchase VPS products on your WooCommerce store, virtual machines are automatically cloned, cloud-init configured, and started in Proxmox VE.
- **Client Portal Shortcode:** Drop `[nevatal_instances]` into any member or customer page to let users view their running servers, IP addresses, and SSH credentials.
- **Hypervisor Health Dashboard:** Monitor Dell PowerEdge R640 node health, CPU utilization, memory consumption, and active instances from WordPress admin.
- **Single Sign-On (SSO):** Log in to Nevatal Cloud using your WordPress credentials.

== Installation ==

1. Upload the `nevatal-cloud-manager` folder to your WordPress `/wp-content/plugins/` directory.
2. Activate the plugin through the 'Plugins' menu in WordPress.
3. Go to the new 'Nevatal Cloud' menu in WordPress Admin.
4. Set your Nevatal API URL (e.g., `https://cloud.nevatal.id` or `http://localhost:8090`) and API Secret Key.
5. Save settings and verify the green "Online & Connected" indicator.

== Shortcodes ==

- `[nevatal_instances]` - Displays the responsive list of virtual servers belonging to the logged-in WordPress user.
