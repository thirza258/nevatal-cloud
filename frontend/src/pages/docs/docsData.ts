export interface DocSection {
  heading: string;
  body: string;
  codeSnippet?: {
    language: string;
    code: string;
    caption?: string;
  };
  table?: {
    headers: string[];
    rows: string[][];
  };
  alert?: {
    type: 'note' | 'tip' | 'important' | 'warning';
    text: string;
  };
}

export interface DocArticle {
  id: string;
  title: string;
  category: string;
  summary: string;
  readTime: string;
  lastUpdated: string;
  sections: DocSection[];
}

export interface DocCategory {
  name: string;
  iconName: string;
  articles: DocArticle[];
}

export const DOCS_CATEGORIES: DocCategory[] = [
  {
    name: 'Getting Started',
    iconName: 'Zap',
    articles: [
      {
        id: 'overview',
        title: 'Platform Overview',
        category: 'Getting Started',
        summary: 'Learn what Nevatal Cloud is, its core capabilities, and how it bridges enterprise hypervisors with modern developer workflows.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Introduction to Nevatal Cloud',
            body: 'Nevatal Cloud is a developer-first cloud platform engineered to run directly on enterprise bare-metal infrastructure. It abstracts the complexity of virtualization hypervisors like Proxmox VE into a clean, minimalist web control plane and REST API. Instead of struggling with raw Linux command lines, firewall rules, or hypervisor dashboards, developers can provision virtual machines, game servers, Android environments, and AI agents in seconds.'
          },
          {
            heading: 'Core Capabilities',
            body: 'The platform provides an integrated suite of cloud services designed for high-density computing and rapid iteration:',
            table: {
              headers: ['Capability', 'Engine / Technology', 'Primary Use Case'],
              rows: [
                ['Compute (VPS)', 'Proxmox VE 8 + Cloud-Init', 'High-performance Linux virtual servers (Ubuntu, Debian, Alpine)'],
                ['WordPress LEMP', 'Nginx 1.26 + PHP 8.3 + MariaDB', 'Production-ready CMS with OPcache and WP-CLI pre-configured'],
                ['Game Servers', 'Managed Containerized Daemons', 'Low-latency multiplayer servers for Minecraft (Java/Bedrock) & Terraria'],
                ['Android Cloud', 'Waydroid LXC Containerization', 'Browser-streamed remote Android 11/13 environments with ADB access'],
                ['App Hosting', 'Git-to-Cloud Builder Pipeline', 'Automated deployments for Node.js, Python, Go, and static websites'],
                ['Automations', 'Managed n8n Runtime', 'Workflow automation with 400+ SaaS triggers and database nodes'],
                ['Object Storage', 'MinIO S3-Compatible Storage', 'High-throughput object buckets with access keys and pre-signed URLs'],
                ['AI Cloud Agent', 'OpenRouter & Model Context Protocol', 'Natural language infrastructure management and DevOps copilot']
              ]
            }
          },
          {
            heading: 'Multi-Tenant Architecture',
            body: 'Nevatal Cloud enforces strict multi-tenant isolation at the hypervisor, network, and database layers. Each user is constrained by allocated vCPU, RAM, and NVMe disk quotas, preventing noisy-neighbor starvation and ensuring deterministic performance.'
          }
        ]
      },
      {
        id: 'quickstart',
        title: '5-Minute Quickstart',
        category: 'Getting Started',
        summary: 'Step-by-step guide to provisioning your first virtual server and establishing SSH connectivity.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Step 1: Account Access',
            body: 'Sign in to the Nevatal Cloud dashboard using your credentials or register a new developer account. For evaluation purposes, pre-configured demo credentials are provided in local environments.'
          },
          {
            heading: 'Step 2: Add Your Public SSH Key',
            body: 'Before launching an instance, upload your public SSH key to ensure secure, passwordless authentication. Nevatal Cloud automatically injects your keys into the instance during cloud-init bootstrap.',
            codeSnippet: {
              language: 'bash',
              code: '# Generate an ed25519 key if you do not have one\nssh-keygen -t ed25519 -C "developer@nevatal"\n\n# Display public key to paste into Dashboard > SSH Keys\ncat ~/.ssh/id_ed25519.pub',
              caption: 'Generating and retrieving an ED25519 public key'
            }
          },
          {
            heading: 'Step 3: Deploy an Instance',
            body: 'Navigate to "Compute" or click "New Instance" in the top bar. Choose your desired operating system (e.g., Ubuntu 24.04 LTS), select a resource plan (e.g., Basic: 2 vCPU / 4 GB RAM), choose your SSH key, and click "Launch Instance".'
          },
          {
            heading: 'Step 4: Connect via SSH',
            body: 'Once the asynchronous job completes (typically under 30 seconds), your instance IP address will appear on the dashboard.',
            codeSnippet: {
              language: 'bash',
              code: 'ssh ubuntu@<INSTANCE_IP_ADDRESS>',
              caption: 'Connecting to your newly created instance'
            },
            alert: {
              type: 'tip',
              text: 'If you ever lose SSH access or misconfigure a firewall rule, you can open the built-in HTML5 noVNC console directly from the Instance Details view.'
            }
          }
        ]
      },
      {
        id: 'architecture',
        title: 'Hardware & Infrastructure',
        category: 'Getting Started',
        summary: 'In-depth overview of the physical Dell PowerEdge R640 nodes, ZFS NVMe storage pools, and hypervisor networking.',
        readTime: '5 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Physical Hardware Specifications',
            body: 'Nevatal Cloud nodes run on enterprise Dell PowerEdge R640 dual-socket servers configured for maximum virtualization density and I/O resilience:',
            table: {
              headers: ['Subsystem', 'Specification', 'Notes'],
              rows: [
                ['Chassis', 'Dell PowerEdge R640 1U Rackmount', 'Dual redundant 750W Titanium PSUs'],
                ['Processors', '2x Intel Xeon Scalable (Gold/Silver)', 'Multi-core hyper-threading with VT-x & EPT'],
                ['Memory', '128 GB - 256 GB ECC Registered DDR4', 'Fault-tolerant ECC error correction'],
                ['Primary Storage', 'Enterprise NVMe SSDs in ZFS Mirror', 'Extreme low latency and high IOPS throughput'],
                ['Network', 'Dual 10GbE SFP+ + Gigabit Management', 'LACP bonded uplink to datacenter distribution switches'],
                ['Hypervisor', 'Proxmox VE 8.x (Linux Kernel 6.8+)', 'KVM virtualization with QEMU guest agent']
              ]
            }
          },
          {
            heading: 'Networking Model',
            body: 'Instances are connected to a software Linux bridge (`vmbr0`) mapped directly to the physical uplinks. Cloud-init automatically negotiates IP addresses through DHCP or assigns static host addresses according to your plan configuration.'
          }
        ]
      }
    ]
  },
  {
    name: 'Compute & VPS',
    iconName: 'Server',
    articles: [
      {
        id: 'instances',
        title: 'Deploying Virtual Machines',
        category: 'Compute & VPS',
        summary: 'How virtual machines are cloned, configured, and managed through the Nevatal Cloud asynchronous worker.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Supported Operating Systems',
            body: 'Nevatal Cloud maintains pre-baked cloud-init base templates optimized for minimal footprint and security:',
            table: {
              headers: ['Distribution', 'Version', 'Base Template VMID', 'Default User'],
              rows: [
                ['Ubuntu Server', '24.04 LTS (Noble Numbat)', '#9000', 'ubuntu'],
                ['Debian Linux', '12 (Bookworm) / 13 (Trixie)', '#9001', 'debian'],
                ['Alpine Linux', '3.20 Standard (Musl/OpenRC)', '#9002', 'alpine'],
                ['WordPress LEMP', 'Ubuntu 24.04 + PHP 8.3 + MariaDB', '#9003', 'ubuntu']
              ]
            }
          },
          {
            heading: 'Asynchronous Provisioning Lifecycle',
            body: 'When you submit a creation request, the backend Go worker coordinates four discrete hypervisor operations:\n1. Full clone of the base template to a new unique VMID.\n2. Dynamic hardware allocation (vCPU cores, memory, and NVMe disk resizing).\n3. Cloud-init user-data generation (hostname, user credentials, authorized keys).\n4. Power-on command and continuous polling of the QEMU Guest Agent for IP assignment.'
          },
          {
            heading: 'Power Controls',
            body: 'You can issue ACPI shutdown, forced stop, reboot, and terminate actions at any time. Changes are reflected in real time across the web control plane.'
          }
        ]
      },
      {
        id: 'cloud-init',
        title: 'Cloud-Init & SSH Key Automation',
        category: 'Compute & VPS',
        summary: 'Understanding metadata injection, custom startup scripts, and secure authentication.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'How Cloud-Init Operates',
            body: 'Cloud-init is the industry-standard multi-distribution package that handles early initialization of a cloud instance. Nevatal Cloud attaches an ISO cloud-init drive to IDE bus 2 (`local-zfs:cloudinit`), containing user-data, meta-data, and network-config.'
          },
          {
            heading: 'Custom User Data Example',
            body: 'You can supply custom Bash or cloud-config scripts during creation to automatically install packages and configure system services upon first boot:',
            codeSnippet: {
              language: 'yaml',
              code: '#cloud-config\npackage_update: true\npackages:\n  - curl\n  - git\n  - htop\n  - ufw\n\nruncmd:\n  - ufw allow 22/tcp\n  - ufw allow 80/tcp\n  - ufw allow 443/tcp\n  - ufw --force enable\n  - echo "Instance successfully provisioned by Nevatal Cloud" > /var/log/provision.log',
              caption: 'Sample cloud-init user-data YAML script'
            }
          }
        ]
      },
      {
        id: 'wordpress-lemp',
        title: 'WordPress 6.7 LEMP Appliance',
        category: 'Compute & VPS',
        summary: 'Deploy an enterprise WordPress site with high-performance Nginx, PHP 8.3-FPM, MariaDB, and WP-CLI.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Pre-Packaged LEMP Stack',
            body: 'The WordPress LEMP appliance provides an instant production environment that bypasses manual Linux software installation. It is built on Ubuntu 24.04 and tuned with OPcache and MariaDB query buffers.'
          },
          {
            heading: 'Appliance Credentials & Access',
            body: 'Upon instance creation, the Instance Details view displays a dedicated "WordPress & LEMP Stack Integration" card containing your generated credentials:',
            table: {
              headers: ['Component', 'Default Port / Location', 'Credential Source'],
              rows: [
                ['Web Interface', 'http://<INSTANCE_IP>/', 'Complete final web installer on first load'],
                ['MariaDB Database', 'localhost:3306 (db: `wordpress`)', 'Randomly generated password in Instance Details'],
                ['Database User', '`wp_user`', 'Granted full privileges on `wordpress` database'],
                ['WP-CLI Utility', '`/usr/local/bin/wp`', 'Available over SSH via standard terminal']
              ]
            }
          },
          {
            heading: 'Managing with WP-CLI',
            body: 'Connect via SSH and run WP-CLI commands directly:',
            codeSnippet: {
              language: 'bash',
              code: '# Check WordPress core version\nwp core version --path=/var/www/html --allow-root\n\n# Install and activate a plugin\nwp plugin install redis-cache --activate --path=/var/www/html --allow-root',
              caption: 'Interacting with WordPress via command line'
            }
          }
        ]
      },
      {
        id: 'console-networking',
        title: 'noVNC Console & Networking',
        category: 'Compute & VPS',
        summary: 'Accessing instances out-of-band using HTML5 noVNC web console and network configurations.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'HTML5 noVNC Console',
            body: 'When SSH is unavailable due to firewall adjustments or failed network interfaces, the noVNC console provides direct out-of-band kernel console access through your browser. Click the "Open VNC Console" button on the instance details page.'
          },
          {
            heading: 'Firewall & Port Rules',
            body: 'All public IP instances have standard outbound access. For inbound traffic, ensure your operating system firewall (`ufw` or `iptables`) permits the target ports for your services (e.g., 22 for SSH, 80/443 for Web).'
          }
        ]
      }
    ]
  },
  {
    name: 'Game Server Hosting',
    iconName: 'Gamepad2',
    articles: [
      {
        id: 'minecraft-java',
        title: 'Minecraft Java Edition',
        category: 'Game Server Hosting',
        summary: 'Deploy Paper, Purpur, Fabric, Forge, or Vanilla servers with graphical properties editor and live console.',
        readTime: '5 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Supported Server Software',
            body: 'Nevatal Cloud automates the installation of popular Minecraft Java server engines:',
            table: {
              headers: ['Software', 'Architecture', 'Best Suited For'],
              rows: [
                ['Paper', 'High-performance Spigot fork', 'General multiplayer, survival communities, plugin support'],
                ['Purpur', 'Paper fork with deep customization', 'Server owners desiring granular gameplay configuration'],
                ['Fabric', 'Modern lightweight modloader', 'Technical players, performance mods (Lithium, FerriteCore)'],
                ['Forge / NeoForge', 'Heavy modding framework', 'Modpacks with custom dimensions, machinery, and items'],
                ['Vanilla', 'Official Mojang server release', 'Pure, unmodified Minecraft experience']
              ]
            }
          },
          {
            heading: 'Visual Server Configuration',
            body: 'Edit essential gameplay parameters without editing raw `server.properties` files:\n- Server MOTD & Player Slots (1-100)\n- Game Mode (Survival, Creative, Adventure)\n- Difficulty (Peaceful, Easy, Normal, Hard)\n- Online Mode (Mojang Auth enforcement)\n- View Distance & Simulation Distance\n- PvP & Whitelist toggles'
          },
          {
            heading: 'Live Console & In-Game Commands',
            body: 'Send commands directly to the running server daemon without logging into the game:',
            codeSnippet: {
              language: 'bash',
              code: '# Common console commands\nop <player_name>\nsay Server restart scheduled in 5 minutes\nwhitelist add <player_name>\nsave-all',
              caption: 'Executing commands in the Game Server Live Console'
            }
          }
        ]
      },
      {
        id: 'minecraft-bedrock',
        title: 'Minecraft Bedrock Edition',
        category: 'Game Server Hosting',
        summary: 'Host official Bedrock Dedicated Servers for mobile, Windows 10/11, and console cross-play.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Bedrock Dedicated Server (BDS)',
            body: 'Nevatal Cloud runs official Bedrock server binaries inside hardened containers. Players on iOS, iPadOS, Android, Windows, and compatible consoles can join using your server address and UDP port 19132.'
          },
          {
            heading: 'Player Permissions & Allowlist',
            body: 'Manage `permissions.json` and `allowlist.json` directly from the dashboard to govern operator access and restrict who can enter your world.'
          }
        ]
      },
      {
        id: 'terraria',
        title: 'Terraria Dedicated Server',
        category: 'Game Server Hosting',
        summary: 'Run vanilla Terraria and tModLoader servers with automated world generation and difficulty modes.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'World Generation Settings',
            body: 'When provisioning a Terraria server, configure world parameters in advance:\n- World Size: Small, Medium, Large\n- Difficulty: Classic, Expert, Master, Journey\n- Server Password: Secure private multiplayer sessions\n- Default Port: 7777 TCP'
          }
        ]
      },
      {
        id: 'game-backups',
        title: 'Backups & World Persistence',
        category: 'Game Server Hosting',
        summary: 'Schedule automated world backups, download archives, and restore prior states with zero data loss.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Automated Snapshots',
            body: 'World directories are saved periodically to local NVMe storage and optionally replicated to MinIO S3 Object Storage. You can take an on-demand snapshot prior to updating game versions or installing new plugins.'
          },
          {
            heading: '1-Click Restoration',
            body: 'To roll back after accidental griefing or corrupt world files, select the desired backup snapshot and click "Restore World". The server daemon will stop gracefully, swap world files, and restart automatically.'
          }
        ]
      }
    ]
  },
  {
    name: 'Android Cloud (Waydroid)',
    iconName: 'Smartphone',
    articles: [
      {
        id: 'android-overview',
        title: 'Waydroid Container Architecture',
        category: 'Android Cloud (Waydroid)',
        summary: 'How containerized Android runs directly on Linux without heavy hardware emulation overhead.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'What is Waydroid?',
            body: 'Waydroid is a container-based approach to boot a full Android operating system on a regular GNU/Linux system like Ubuntu. Unlike traditional emulators (which translate CPU instructions through slow QEMU virtualization), Waydroid shares the host Linux kernel directly via LXC namespaces and cgroups.'
          },
          {
            heading: 'Performance Advantage',
            body: 'By leveraging the host Dell PowerEdge R640 Xeon CPUs directly, Waydroid achieves near bare-metal execution speeds, minimal RAM overhead, and rapid cold boots (< 8 seconds).'
          }
        ]
      },
      {
        id: 'android-streaming',
        title: 'Browser Remote Touch Streaming',
        category: 'Android Cloud (Waydroid)',
        summary: 'Interact with Android apps in real time directly inside your web browser via low-latency streaming.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Web-Based Display Session',
            body: 'Every Android instance exposes an interactive display canvas streaming at up to 60 FPS over WebSockets/WebRTC. Mouse movements translate into touch events, clicks act as finger taps, and keypresses are sent to the Android software input system.'
          },
          {
            heading: 'Configuring Resolution & Density',
            body: 'You can adjust virtual screen dimensions in Instance Settings:',
            table: {
              headers: ['Profile', 'Resolution (W x H)', 'Recommended DPI', 'Use Case'],
              rows: [
                ['Phone Standard', '720 x 1280 (HD)', '240 DPI', 'Mobile app testing and automation'],
                ['Phone High-Res', '1080 x 1920 (FHD)', '320 DPI', 'High-density UI layout verification'],
                ['Tablet Landscape', '1920 x 1080 (FHD)', '280 DPI', 'Landscape dashboard and productivity tools']
              ]
            }
          }
        ]
      },
      {
        id: 'adb-apps',
        title: 'ADB Connectivity & App Testing',
        category: 'Android Cloud (Waydroid)',
        summary: 'Connect Android Debug Bridge (ADB) over network for automated testing, APK installation, and logcat debugging.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Connecting via Network ADB',
            body: 'Each Android Cloud instance exposes an ADB daemon on port 5555. You can connect from your local terminal or CI/CD runner:',
            codeSnippet: {
              language: 'bash',
              code: '# Connect to remote Android instance\nadb connect <INSTANCE_IP>:5555\n\n# Verify connected devices\nadb devices\n\n# Install an APK package\nadb install my-test-app.apk\n\n# Stream logcat output\nadb logcat -v time *:E',
              caption: 'Interacting with Android instance via ADB'
            }
          },
          {
            heading: 'Automated CI/CD Test Execution',
            body: 'Run Appium, Espresso, or Maestro automated UI tests against cloud-hosted Android instances without maintaining physical test devices.'
          }
        ]
      }
    ]
  },
  {
    name: 'Apps, Automations & AI',
    iconName: 'Workflow',
    articles: [
      {
        id: 'git-deploy',
        title: 'Git Web App Deployments',
        category: 'Apps, Automations & AI',
        summary: 'Deploy Node.js, Python, Go, and static applications straight from GitHub and GitLab repositories.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Continuous Deployment Workflow',
            body: 'Link your Git repository and branch. The platform automatically detects the project framework, executes the build pipeline, isolates dependencies, and configures reverse proxy routing.'
          },
          {
            heading: 'Environment Variables',
            body: 'Configure secure runtime secrets and environment keys in the App Settings tab. Variables are injected into the runtime container securely without writing credentials to disk.'
          }
        ]
      },
      {
        id: 'n8n-automation',
        title: 'Managed n8n Workflow Automation',
        category: 'Apps, Automations & AI',
        summary: 'Run high-throughput workflow automations with 400+ pre-built integration nodes and webhook triggers.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Dedicated n8n Instance',
            body: 'Nevatal Cloud provisions a dedicated n8n instance backed by persistent SQLite or PostgreSQL storage. You retain complete ownership of your workflow data, API credentials, and webhooks.'
          },
          {
            heading: 'Webhooks & Scheduled Triggers',
            body: 'Receive inbound webhooks from Stripe, GitHub, Slack, or IoT sensors, process payloads with JavaScript, and trigger automated cloud scaling actions.'
          }
        ]
      },
      {
        id: 'openclaw-agents',
        title: 'OpenClaw Autonomous AI Agents',
        category: 'Apps, Automations & AI',
        summary: 'Deploy persistent AI agent runtimes equipped with tools, memory, and scheduled task triggers.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Agent Framework',
            body: 'OpenClaw provides a sandboxed environment for LLM-driven agents capable of autonomous multi-step reasoning, external tool execution, and state persistence.'
          }
        ]
      }
    ]
  },
  {
    name: 'Storage & APIs',
    iconName: 'Database',
    articles: [
      {
        id: 's3-storage',
        title: 'S3-Compatible Object Storage',
        category: 'Storage & APIs',
        summary: 'High-throughput MinIO object buckets with AWS S3 API compatibility and web file browser.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Connecting with AWS SDKs',
            body: 'Nevatal Cloud S3 buckets are compatible with the official AWS SDK, `aws-cli`, `rclone`, and `s3cmd`. Simply configure your custom endpoint URL:',
            codeSnippet: {
              language: 'bash',
              code: '# Configure AWS CLI with Nevatal Cloud endpoint\naws configure set aws_access_key_id <YOUR_ACCESS_KEY>\naws configure set aws_secret_access_key <YOUR_SECRET_KEY>\naws configure set default.region us-east-1\n\n# List buckets\naws --endpoint-url https://cloud.nevatal.id:9000 s3 ls\n\n# Upload a file\naws --endpoint-url https://cloud.nevatal.id:9000 s3 cp dataset.tar.gz s3://my-bucket/',
              caption: 'Using AWS CLI with Nevatal Cloud Object Storage'
            }
          },
          {
            heading: 'Access Keys & Permissions',
            body: 'Create read-only or read-write access keys from "Storage > Access Keys". Revoke keys instantly when rotating application credentials.'
          }
        ]
      },
      {
        id: 'rest-api',
        title: 'Nevatal REST API v1 Reference',
        category: 'Storage & APIs',
        summary: 'Programmatically manage cloud instances, retrieve telemetry, and automate deployments via REST API.',
        readTime: '5 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Authentication & Headers',
            body: 'Authenticate all API requests by providing your session JWT token in the `Authorization` header:',
            codeSnippet: {
              language: 'http',
              code: 'GET /api/v1/instances HTTP/1.1\nHost: cloud.nevatal.id\nAuthorization: Bearer <YOUR_SESSION_JWT>\nContent-Type: application/json',
              caption: 'Sample REST API Request Header'
            }
          },
          {
            heading: 'Core Endpoints',
            body: 'Key endpoints available for automation and CI/CD pipelines:',
            table: {
              headers: ['Method', 'Endpoint', 'Description'],
              rows: [
                ['GET', '/api/v1/dashboard', 'Retrieve quota usage, active counts, and recent instances'],
                ['GET', '/api/v1/instances', 'List all compute virtual machines for the user'],
                ['POST', '/api/v1/instances', 'Provision a new virtual server from template'],
                ['POST', '/api/v1/instances/:id/reboot', 'Trigger graceful ACPI or hard reboot'],
                ['POST', '/api/v1/instances/:id/stop', 'Halt running virtual machine'],
                ['POST', '/api/v1/instances/:id/start', 'Power on stopped virtual machine'],
                ['DELETE', '/api/v1/instances/:id', 'Destroy instance and release quota allocations'],
                ['GET', '/api/v1/game-servers', 'List managed game servers and statuses'],
                ['POST', '/api/v1/game-servers', 'Create a new Minecraft or Terraria game server'],
                ['GET', '/api/v1/android/instances', 'List Waydroid Android instances']
              ]
            }
          }
        ]
      },
      {
        id: 'mcp-protocol',
        title: 'Model Context Protocol (MCP) Integration',
        category: 'Storage & APIs',
        summary: 'Connect your cloud infrastructure directly to AI agents in Claude Desktop, Cursor, and IDEs.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'What is MCP?',
            body: 'Model Context Protocol (MCP) is an open standard that allows LLMs to interact with external tools and databases safely. Nevatal Cloud provides a native MCP server exposing secure tools for querying instances, starting/stopping workloads, and inspecting telemetry.'
          },
          {
            heading: 'Connecting Claude Desktop or Cursor',
            body: 'Add Nevatal Cloud to your `claude_desktop_config.json`:',
            codeSnippet: {
              language: 'json',
              code: '{\n  "mcpServers": {\n    "nevatal-cloud": {\n      "command": "npx",\n      "args": ["-y", "@nevatal/mcp-server"],\n      "env": {\n        "NEVATAL_API_ENDPOINT": "https://cloud.nevatal.id/api/v1",\n        "NEVATAL_API_TOKEN": "<YOUR_API_TOKEN>"\n      }\n    }\n  }\n}',
              caption: 'Configuring Claude Desktop MCP client'
            }
          }
        ]
      }
    ]
  },
  {
    name: 'Infrastructure & Admin',
    iconName: 'Shield',
    articles: [
      {
        id: 'proxmox-setup',
        title: 'Proxmox VE Hypervisor Setup',
        category: 'Infrastructure & Admin',
        summary: 'How administrators connect physical Proxmox hypervisor nodes to the Nevatal Cloud backend.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Creating the Proxmox API Token',
            body: 'Nevatal Cloud uses dedicated Proxmox API tokens for zero-password hypervisor orchestration. In your Proxmox Web GUI:\n1. Navigate to Datacenter > Permissions > Users > Add: `nevatal-cloud@pve`.\n2. Navigate to API Tokens > Add: token ID `api-token`.\n3. Assign `PVEVMAdmin` and `PVEDatastoreUser` permissions to `/`.'
          },
          {
            heading: 'Environment Configuration',
            body: 'Supply the credentials in `.env`:',
            codeSnippet: {
              language: 'bash',
              code: 'PROXMOX_MOCK=false\nPROXMOX_HOST=192.168.1.100\nPROXMOX_PORT=8006\nPROXMOX_TOKEN_ID=nevatal-cloud@pve!api-token\nPROXMOX_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\nPROXMOX_NODE_NAME=pve-dell-r640\nPROXMOX_STORAGE=local-zfs\nPROXMOX_VERIFY_TLS=false',
              caption: 'Configuring Proxmox VE connection in .env'
            }
          }
        ]
      },
      {
        id: 'cloudinit-templates',
        title: 'Creating Base VM Templates (#9000+)',
        category: 'Infrastructure & Admin',
        summary: 'How base cloud-init templates are prepared and converted into VM templates on Proxmox.',
        readTime: '4 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'CLI Template Generation Workflow',
            body: 'Execute the following commands on the Proxmox host via SSH to bake an official Ubuntu 24.04 Cloud-Init template:',
            codeSnippet: {
              language: 'bash',
              code: 'wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img\n\nqm create 9000 --name "ubuntu-2404-template" --memory 2048 --cores 2 --net0 virtio,bridge=vmbr0\nqm importdisk 9000 noble-server-cloudimg-amd64.img local-zfs\nqm set 9000 --scsihw virtio-scsi-pci --scsi0 local-zfs:vm-9000-disk-0\nqm set 9000 --boot c --bootdisk scsi0\nqm set 9000 --ide2 local-zfs:cloudinit\nqm set 9000 --serial0 socket --vga serial0\nqm set 9000 --agent enabled=1\nqm template 9000',
              caption: 'Proxmox shell commands to create VMID #9000 template'
            }
          }
        ]
      },
      {
        id: 'iso-catalog',
        title: 'Uploading ISO Media',
        category: 'Infrastructure & Admin',
        summary: 'Manage raw installation ISO images on Proxmox datastores via the Administrator ISO Catalog.',
        readTime: '3 min read',
        lastUpdated: 'October 2026',
        sections: [
          {
            heading: 'Media Catalog',
            body: 'Administrators can upload standard operating system ISOs directly through the web UI at `/admin/isos`. Uploaded ISOs reside in the Proxmox `local:iso` datastore and can be mounted to virtual machines for manual installation.'
          }
        ]
      }
    ]
  }
];

export const ALL_DOC_ARTICLES: DocArticle[] = DOCS_CATEGORIES.flatMap(c => c.articles);

export function getArticleById(id: string): DocArticle | undefined {
  return ALL_DOC_ARTICLES.find(a => a.id === id);
}
