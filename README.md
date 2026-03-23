# KubeKit

A terminal UI toolkit for Kubernetes and AWS workflows. Navigate clusters, manage pods, forward database tunnels, and handle AWS SSO — all from an interactive menu.

## Features

- **Pod Management** — list, logs, shell, inspect
- **Deployments** — browse, scale, restart
- **Resources** — namespaces, services, ingress
- **Cluster** — context switching, node inspection
- **Database Tunnels** — forward via socat pod with multi-port support
- **Port Forwarding** — interactive service port forwarding
- **AWS** — SSO login, EKS context setup, S3 browsing
- **Animated TUI** — clean terminal UI with spinners and smooth navigation

## Install

### Homebrew (coming soon)

```bash
brew tap Tyka95/kube-kit
brew install kubekit
```

### Manual

```bash
git clone https://github.com/Tyka95/kube-kit.git
cd kube-kit
chmod +x kubekit.sh
./kubekit.sh
```

## Dependencies

| Dependency | Required | Install |
|-----------|----------|---------|
| `gum` | Yes | `brew install gum` |
| `kubectl` | Yes | `brew install kubectl` |
| `aws` | For AWS features | `brew install awscli` |
| `fzf` | Optional (enhanced filtering) | `brew install fzf` |

## Usage

```bash
kubekit.sh
```

Use arrow keys to navigate menus, Enter to select, Escape to go back.

```bash
kubekit.sh --version   # Print version
```

## License

[MIT](LICENSE)
