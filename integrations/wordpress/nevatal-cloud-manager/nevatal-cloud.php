<?php
/**
 * Plugin Name: Nevatal Cloud VPS Manager & Orchestrator
 * Plugin URI: https://cloud.nevatal.id
 * Description: Seamlessly integrate WordPress and WooCommerce with Nevatal Cloud Proxmox VE control plane. Automate VPS provisioning on checkout, manage virtual servers, and embed customer cloud dashboards via shortcodes.
 * Version: 1.0.0
 * Author: Nevatal
 * Author URI: https://nevatal.id
 * License: GPL v2 or later
 * Text Domain: nevatal-cloud
 */

if (!defined('ABSPATH')) {
    exit;
}

class NevatalCloudManager {
    private static $instance = null;
    private $option_key = 'nevatal_cloud_settings';

    public static function get_instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        add_action('admin_menu', array($this, 'add_admin_menu'));
        add_action('admin_init', array($this, 'register_settings'));
        add_shortcode('nevatal_instances', array($this, 'render_instances_shortcode'));
        add_shortcode('nevatal_cloud', array($this, 'render_instances_shortcode'));

        // WooCommerce order completion hook
        add_action('woocommerce_order_status_completed', array($this, 'handle_woocommerce_order'), 10, 1);

        // AJAX handlers for instance control and test connection
        add_action('wp_ajax_nevatal_test_connection', array($this, 'ajax_test_connection'));
        add_action('wp_ajax_nevatal_control_instance', array($this, 'ajax_control_instance'));
    }

    public function get_api_url() {
        $options = get_option($this->option_key, array());
        $url = !empty($options['api_url']) ? rtrim($options['api_url'], '/') : 'http://localhost:8090';
        return $url;
    }

    public function get_api_key() {
        $options = get_option($this->option_key, array());
        return !empty($options['api_key']) ? $options['api_key'] : 'wp_nevatal_cloud_api_key_secret_2026';
    }

    public function add_admin_menu() {
        add_menu_page(
            'Nevatal Cloud',
            'Nevatal Cloud',
            'manage_options',
            'nevatal-cloud',
            array($this, 'render_admin_page'),
            'dashicons-cloud',
            30
        );
    }

    public function register_settings() {
        register_setting('nevatal_cloud_group', $this->option_key);
    }

    public function call_api($endpoint, $method = 'GET', $body = null) {
        $url = $this->get_api_url() . $endpoint;
        $args = array(
            'method'  => $method,
            'timeout' => 15,
            'headers' => array(
                'Content-Type'  => 'application/json',
                'X-Nevatal-Key' => $this->get_api_key(),
                'User-Agent'    => 'WordPress-NevatalCloud-Plugin/1.0.0',
            ),
        );

        if ($body !== null) {
            $args['body'] = json_encode($body);
        }

        $response = wp_remote_request($url, $args);
        if (is_wp_error($response)) {
            return array('success' => false, 'error' => $response->get_error_message());
        }

        $code = wp_remote_retrieve_response_code($response);
        $data = json_decode(wp_remote_retrieve_body($response), true);

        return array(
            'success' => ($code >= 200 && $code < 300),
            'code'    => $code,
            'data'    => $data,
        );
    }

    public function ajax_test_connection() {
        check_ajax_referer('nevatal_admin_nonce', 'nonce');
        if (!current_user_can('manage_options')) {
            wp_send_json_error('Unauthorized');
        }

        $res = $this->call_api('/api/v1/wordpress/status');
        if ($res['success']) {
            wp_send_json_success($res['data']);
        } else {
            wp_send_json_error(isset($res['error']) ? $res['error'] : 'Failed to reach Nevatal Cloud API');
        }
    }

    public function ajax_control_instance() {
        if (!is_user_logged_in()) {
            wp_send_json_error('You must be logged in.');
        }

        $instance_id = sanitize_text_field($_POST['instance_id']);
        $power_action = sanitize_text_field($_POST['power_action']);

        $res = $this->call_api('/api/v1/webhooks/wordpress', 'POST', array(
            'action'       => 'control_instance',
            'instance_id'  => $instance_id,
            'power_action' => $power_action,
        ));

        if ($res['success']) {
            wp_send_json_success($res['data']);
        } else {
            wp_send_json_error(isset($res['data']['error']['message']) ? $res['data']['error']['message'] : 'Action failed');
        }
    }

    /**
     * WooCommerce automated VPS provisioning on completed order
     */
    public function handle_woocommerce_order($order_id) {
        if (!function_exists('wc_get_order')) {
            return;
        }

        $order = wc_get_order($order_id);
        if (!$order) {
            return;
        }

        $customer_email = $order->get_billing_email();
        $customer_name  = $order->get_billing_first_name() . ' ' . $order->get_billing_last_name();

        // Send provisioning request to Nevatal Cloud API
        $payload = array(
            'action'      => 'provision_vps',
            'user_email'  => $customer_email,
            'user_name'   => trim($customer_name),
            'order_id'    => (string)$order_id,
            'custom_name' => 'vps-order-' . $order_id,
        );

        $res = $this->call_api('/api/v1/webhooks/wordpress', 'POST', $payload);
        if ($res['success'] && !empty($res['data']['instance_id'])) {
            $order->add_order_note(
                sprintf(
                    'Nevatal Cloud VPS provisioned successfully! Instance ID: %s (Job: %s). User email: %s',
                    $res['data']['instance_id'],
                    $res['data']['job_id'],
                    $customer_email
                )
            );
        } else {
            $order->add_order_note('Nevatal Cloud VPS Provisioning encountered an issue: ' . json_encode($res));
        }
    }

    public function render_admin_page() {
        $options = get_option($this->option_key, array());
        $status_res = $this->call_api('/api/v1/wordpress/status');
        ?>
        <div class="wrap" style="max-width: 1000px; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">
            <h1 style="display: flex; align-items: center; gap: 10px; margin-bottom: 20px;">
                <span class="dashicons dashicons-cloud" style="font-size: 32px; width: 32px; height: 32px; color: #2271b1;"></span>
                Nevatal Cloud VPS Orchestrator
            </h1>

            <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 20px;">
                <!-- Settings Panel -->
                <div style="background: #fff; padding: 25px; border-radius: 12px; border: 1px solid #ccd0d4; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
                    <h2 style="margin-top: 0; font-size: 16px; border-bottom: 1px solid #eee; padding-bottom: 10px;">Connection Settings</h2>
                    
                    <form method="post" action="options.php">
                        <?php settings_fields('nevatal_cloud_group'); ?>
                        
                        <table class="form-table" role="presentation">
                            <tr>
                                <th scope="row"><label for="api_url">Nevatal API Endpoint</label></th>
                                <td>
                                    <input name="<?php echo esc_attr($this->option_key); ?>[api_url]" type="url" id="api_url" value="<?php echo esc_attr($this->get_api_url()); ?>" class="regular-text" placeholder="http://localhost:8090" />
                                    <p class="description">URL of your Nevatal Cloud Control Plane (e.g. <code>https://cloud.nevatal.id</code> or <code>http://localhost:8090</code>).</p>
                                </td>
                            </tr>
                            <tr>
                                <th scope="row"><label for="api_key">Integration Secret Key</label></th>
                                <td>
                                    <input name="<?php echo esc_attr($this->option_key); ?>[api_key]" type="password" id="api_key" value="<?php echo esc_attr($this->get_api_key()); ?>" class="regular-text" />
                                    <p class="description">Matches <code>WORDPRESS_API_KEY</code> in Nevatal Cloud environment.</p>
                                </td>
                            </tr>
                        </table>

                        <?php submit_button('Save Configuration'); ?>
                    </form>

                    <hr style="margin: 25px 0; border: 0; border-top: 1px solid #eee;" />

                    <h3>Frontend Embedding Shortcode</h3>
                    <p>Embed the self-service cloud server management interface into any WordPress page or member portal:</p>
                    <code style="background: #f0f0f1; padding: 8px 12px; display: inline-block; border-radius: 6px; font-size: 14px; font-weight: 600;">[nevatal_instances]</code>
                </div>

                <!-- Status Sidebar -->
                <div>
                    <div style="background: #fff; padding: 20px; border-radius: 12px; border: 1px solid #ccd0d4; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
                        <h2 style="margin-top: 0; font-size: 15px; border-bottom: 1px solid #eee; padding-bottom: 10px;">Proxmox Hypervisor Status</h2>
                        
                        <?php if ($status_res['success']): 
                            $node = $status_res['data']['node_health'];
                        ?>
                            <div style="margin-bottom: 12px;">
                                <span style="display: inline-block; width: 10px; height: 10px; background: #46b450; border-radius: 50%; margin-right: 6px;"></span>
                                <strong style="color: #2e7d32;">Online & Connected</strong>
                            </div>
                            <p style="font-size: 13px; margin: 4px 0;"><strong>Target Node:</strong> <code><?php echo esc_html($status_res['data']['hypervisor_node']); ?></code></p>
                            <p style="font-size: 13px; margin: 4px 0;"><strong>Active VMs:</strong> <?php echo intval($status_res['data']['active_instances']); ?></p>
                            <p style="font-size: 13px; margin: 4px 0;"><strong>Kernel:</strong> <?php echo esc_html(substr($node['kversion'], 0, 22)); ?>...</p>
                            <p style="font-size: 13px; margin: 4px 0;"><strong>Memory:</strong> <?php echo round($node['memory']['used'] / (1024*1024*1024), 1); ?> GB / <?php echo round($node['memory']['total'] / (1024*1024*1024), 1); ?> GB</p>
                        <?php else: ?>
                            <div style="color: #dc3232; margin-bottom: 10px;">
                                <span style="display: inline-block; width: 10px; height: 10px; background: #dc3232; border-radius: 50%; margin-right: 6px;"></span>
                                <strong>Disconnected</strong>
                            </div>
                            <p style="font-size: 12px; color: #666;">Unable to reach Nevatal Cloud API. Ensure backend is running and URL is correct.</p>
                        <?php endif; ?>
                    </div>
                </div>
            </div>
        </div>
        <?php
    }

    public function render_instances_shortcode($atts) {
        if (!is_user_logged_in()) {
            return '<div style="padding: 15px; background: #f8d7da; color: #721c24; border-radius: 8px;">Please log in to view your cloud instances.</div>';
        }

        $current_user = wp_get_current_user();
        $email = $current_user->user_email;

        $res = $this->call_api('/api/v1/webhooks/wordpress', 'POST', array(
            'action'     => 'list_instances',
            'user_email' => $email,
        ));

        $instances = ($res['success'] && isset($res['data']['data'])) ? $res['data']['data'] : array();

        ob_start();
        ?>
        <div class="nevatal-customer-portal" style="font-family: inherit; margin: 20px 0;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                <h3 style="margin: 0;">My Virtual Servers (<?php echo count($instances); ?>)</h3>
                <span style="font-size: 12px; color: #666;">Powered by Nevatal Cloud</span>
            </div>

            <?php if (empty($instances)): ?>
                <div style="padding: 30px; text-align: center; background: #f9f9f9; border: 1px dashed #ddd; border-radius: 8px; color: #666;">
                    You do not have any active virtual servers provisioned yet.
                </div>
            <?php else: ?>
                <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 15px;">
                    <?php foreach ($instances as $inst): ?>
                        <div style="background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
                            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
                                <strong style="font-size: 15px;"><?php echo esc_html($inst['name']); ?></strong>
                                <span style="font-size: 11px; padding: 2px 8px; border-radius: 12px; font-weight: 600; text-transform: uppercase; background: <?php echo $inst['status'] === 'running' ? '#dcfce7; color: #15803d;' : '#f1f5f9; color: #475569;'; ?>">
                                    <?php echo esc_html($inst['status']); ?>
                                </span>
                            </div>

                            <div style="font-size: 12px; color: #64748b; margin-bottom: 12px; font-family: monospace;">
                                IP: <strong style="color: #0f172a;"><?php echo esc_html(!empty($inst['private_ip']) ? $inst['private_ip'] : 'Allocating lease...'); ?></strong><br>
                                Specs: <?php echo intval($inst['vcpus']); ?> vCPU &bull; <?php echo round($inst['memory_mb']/1024); ?> GB RAM &bull; <?php echo intval($inst['disk_gb']); ?> GB SSD
                            </div>

                            <div style="border-top: 1px solid #f1f5f9; padding-top: 10px; display: flex; gap: 8px;">
                                <button style="flex: 1; padding: 6px; font-size: 12px; border-radius: 6px; border: 1px solid #cbd5e1; background: #f8fafc; cursor: pointer;" onclick="alert('Connect via SSH: ssh <?php echo esc_js($inst['ssh_username']); ?>@<?php echo esc_js(!empty($inst['private_ip']) ? $inst['private_ip'] : 'IP'); ?>')">
                                    SSH Info
                                </button>
                                <a href="<?php echo esc_url($this->get_api_url() . ':3080/dashboard/instances'); ?>" target="_blank" style="flex: 1; text-align: center; text-decoration: none; padding: 6px; font-size: 12px; border-radius: 6px; background: #2563eb; color: #fff; font-weight: 500;">
                                    Console
                                </a>
                            </div>
                        </div>
                    <?php endforeach; ?>
                </div>
            <?php endif; ?>
        </div>
        <?php
        return ob_get_clean();
    }
}

NevatalCloudManager::get_instance();
