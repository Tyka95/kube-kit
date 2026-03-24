# Changelog

## [0.1.5](https://github.com/Tyka95/kube-kit/compare/v0.1.4...v0.1.5) (2026-03-24)


### Features

* auto-adapt menu to terminal size on resize ([4bfefbf](https://github.com/Tyka95/kube-kit/commit/4bfefbf3d98b459f9b3303d498c6c6dae67a6f70))


### Bug Fixes

* full screen clear on terminal resize to prevent corruption ([124339c](https://github.com/Tyka95/kube-kit/commit/124339c5873b820fcd300c00cf692a352d94e0cf))
* override all gum pink defaults with cyan theme ([f61974c](https://github.com/Tyka95/kube-kit/commit/f61974c910e72e3ba2e119a60bd26c1b6c81de26))
* proper alt screen reset and line clearing on terminal resize ([fae7576](https://github.com/Tyka95/kube-kit/commit/fae7576dd25a5b02b353ea1ee215541373fc91d2))
* rewrite resize handling with debounce, atomic writes, and bash read ([c9b78c7](https://github.com/Tyka95/kube-kit/commit/c9b78c7c86ce45738eeadf06b219bdc9dc637838))
* ui improvements ([4329465](https://github.com/Tyka95/kube-kit/commit/4329465096212057322a8969c77a33793087acd6))


### Reverts

* restore original ui.sh, remove broken resize handling ([ba8d9b3](https://github.com/Tyka95/kube-kit/commit/ba8d9b322ee0d74ef556bb6825937dbf65516c93))

## [0.1.4](https://github.com/Tyka95/kube-kit/compare/v0.1.3...v0.1.4) (2026-03-23)


### Bug Fixes

* trigger homebrew update only on published event ([bc88b3b](https://github.com/Tyka95/kube-kit/commit/bc88b3b343d1b8bfc5ba728113918588bf805fc0))

## [0.1.3](https://github.com/Tyka95/kube-kit/compare/v0.1.2...v0.1.3) (2026-03-23)


### Bug Fixes

* use PAT for release-please to trigger downstream workflows ([28215df](https://github.com/Tyka95/kube-kit/commit/28215df585d2e78ce09e32695f5af1b55bcf78c2))

## [0.1.2](https://github.com/Tyka95/kube-kit/compare/v0.1.1...v0.1.2) (2026-03-23)


### Bug Fixes

* improve homebrew tap update trigger ([9f35bf5](https://github.com/Tyka95/kube-kit/commit/9f35bf56ee02a64c9ed61ea2e8681a60e9a6c61c))

## [0.1.1](https://github.com/Tyka95/kube-kit/compare/v0.1.0...v0.1.1) (2026-03-23)


### Features

* initial release of KubeKit ([1b07392](https://github.com/Tyka95/kube-kit/commit/1b07392392ad1819d32639a83380b79a0f63f0fe))

## [0.1.0](https://github.com/Tyka95/kube-kit/commits/main) (2026-03-23)

### Features

* Initial release of KubeKit
* Pod management (list, logs, shell, inspect)
* Deployment management (browse, scale, restart)
* Resource browsing (namespaces, services, ingress)
* Cluster operations (context switching, node inspection)
* Database tunnel forwarding via socat pod
* Interactive port forwarding
* AWS integration (SSO, EKS, S3)
* Animated terminal UI with menus and spinners
