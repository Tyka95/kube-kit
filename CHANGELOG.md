# Changelog

## [0.1.14](https://github.com/Tyka95/kube-kit/compare/v0.1.13...v0.1.14) (2026-05-19)


### Features

* Add config/state, paging, UI and UX tweaks ([27d30be](https://github.com/Tyka95/kube-kit/commit/27d30befdbaf0f158c987a0e94868b7cc09cacd2))
* auto-adapt menu to terminal size on resize ([4bfefbf](https://github.com/Tyka95/kube-kit/commit/4bfefbf3d98b459f9b3303d498c6c6dae67a6f70))
* AWS session state, footer status bar, RDS auto-discovery, v0.2 chrome ([#9](https://github.com/Tyka95/kube-kit/issues/9)) ([00cf51b](https://github.com/Tyka95/kube-kit/commit/00cf51ba5be59ad3ca353ed71acd18f8febc9514))
* **go:** port awssession + rds + kctx + config packages ([ae1beb9](https://github.com/Tyka95/kube-kit/commit/ae1beb9b95bcb8c4d90592283292c069108396cf))
* **go:** scaffold Bubble Tea project structure ([ade3ee3](https://github.com/Tyka95/kube-kit/commit/ade3ee32db8a4b7f32a9b5dd73a73570364ed2d7))
* **go:** wire all screens, tunnel + commands packages, help overlay ([dbcfb9f](https://github.com/Tyka95/kube-kit/commit/dbcfb9ff5a0e79169cb6e1b982cfeb89bacedbb0))
* initial release of KubeKit ([1b07392](https://github.com/Tyka95/kube-kit/commit/1b07392392ad1819d32639a83380b79a0f63f0fe))
* **release:** goreleaser pipeline, CI workflow, release-please switch to go module ([a0f00ce](https://github.com/Tyka95/kube-kit/commit/a0f00ce6a102619ca51a6dc23a803051b55f0aa3))
* **theme:** ShimmerGlowAt foreground helper for picker row glow ([f4c0282](https://github.com/Tyka95/kube-kit/commit/f4c028297b92cafcd6dd337679846254397116d6))
* **theme:** switch palette from Tokyo Night to Nord ([8b835fa](https://github.com/Tyka95/kube-kit/commit/8b835fa2c56c40c05a62d08313dd0e064c46b6fb))
* **tui:** auto-copy SSO verification code to clipboard ([71a5780](https://github.com/Tyka95/kube-kit/commit/71a5780f4b7cd5485eecf6904e7333efc000a7b7))
* **tui:** auto-open SSO login URL in the user's default browser ([d68c1eb](https://github.com/Tyka95/kube-kit/commit/d68c1ebac717266ed066ea605fc8595513dc8535))
* **tui:** continuous shimmer on the resting selected row ([c049cd0](https://github.com/Tyka95/kube-kit/commit/c049cd01cda416601b56595de14f9d6b8e0be96f))
* **tui:** embed SSO login flow inside the TUI (no more terminal hand-off) ([df5ce42](https://github.com/Tyka95/kube-kit/commit/df5ce42ce09ea83c6f1e8ce90d8a01c4912c573f))
* **tui:** fix stale AWS header after sso login + visible animations ([69d6ef8](https://github.com/Tyka95/kube-kit/commit/69d6ef80613c7a880febc86db729e7d41592e7d1))
* **tui:** floating '❯' caret marker on selected row + hide terminal cursor ([ae1be45](https://github.com/Tyka95/kube-kit/commit/ae1be450e651fcc4cbed7f75d890e897cb695da7))
* **tui:** pre-flight SSO login screen before suspending ([972dbe5](https://github.com/Tyka95/kube-kit/commit/972dbe58cb5546da424d75617570adb2f9f0a4de))
* **tui:** real pod listing via kubectl ([3308161](https://github.com/Tyka95/kube-kit/commit/33081610dd5e3f74452f2f74f5f2315aba07d6c1))
* **tui:** real Pods + Deployments actions ([8db356b](https://github.com/Tyka95/kube-kit/commit/8db356bc18e621428c13891892a809b9b86a1439))
* **tui:** real Resources actions (Namespaces / Services / Ingress) ([e52518a](https://github.com/Tyka95/kube-kit/commit/e52518aec6ae1a489f6017714ce4b42a896dd561))
* **tui:** route PickerHelpMsg and PickerCommandMsg at the app level ([11dc521](https://github.com/Tyka95/kube-kit/commit/11dc521d95668da0d56ae37fbf0468232bf26dd8))
* **tui:** selection-move flash animation + accent bar + styled callouts ([09e5696](https://github.com/Tyka95/kube-kit/commit/09e56963da59315663f4a1a7992f10f9c2bd8e89))
* **tui:** smooth RGB-interpolated selection fade, drop ugly slide ([619b8b6](https://github.com/Tyka95/kube-kit/commit/619b8b6c3b11b8589b4a6b71ce6e4491bab77c67))


### Bug Fixes

* addional UI improvements and stability ([1e844bf](https://github.com/Tyka95/kube-kit/commit/1e844bf66eae710fe8fd79cbf94d3aece60fb1ef))
* **awssession:** resolve region from cluster URL when context is an alias; rds skip on empty region ([c81b899](https://github.com/Tyka95/kube-kit/commit/c81b8992b9d1a4ccb6bbbc724e7b200712900eaf))
* **aws:** show short cluster name in context picker, not full ARN ([219de9b](https://github.com/Tyka95/kube-kit/commit/219de9b5d05af130e80d4da37401f4703e079152))
* **aws:** show short cluster name in context picker, not full ARN ([c0fd9bf](https://github.com/Tyka95/kube-kit/commit/c0fd9bf56dec5abc6517eb932846f23ac18be1e9))
* full screen clear on terminal resize to prevent corruption ([124339c](https://github.com/Tyka95/kube-kit/commit/124339c5873b820fcd300c00cf692a352d94e0cf))
* improve homebrew tap update trigger ([9f35bf5](https://github.com/Tyka95/kube-kit/commit/9f35bf56ee02a64c9ed61ea2e8681a60e9a6c61c))
* improved aliases ([cffb91a](https://github.com/Tyka95/kube-kit/commit/cffb91afdead140e70be044970b45efed04ec201))
* override all gum pink defaults with cyan theme ([f61974c](https://github.com/Tyka95/kube-kit/commit/f61974c910e72e3ba2e119a60bd26c1b6c81de26))
* **picker:** rune-safe row render with foreground-only shimmer glow ([9265339](https://github.com/Tyka95/kube-kit/commit/9265339be78220b0cebd0d038d536e966aaa8ac6))
* proper alt screen reset and line clearing on terminal resize ([fae7576](https://github.com/Tyka95/kube-kit/commit/fae7576dd25a5b02b353ea1ee215541373fc91d2))
* **release:** create release as draft so assets can upload ([19ba85c](https://github.com/Tyka95/kube-kit/commit/19ba85cabcbb30cda5121309e9db398797b0f395))
* **release:** create release as draft so assets can upload ([2ecb8df](https://github.com/Tyka95/kube-kit/commit/2ecb8df181b24994540800d13d791bb72cf79162))
* rewrite resize handling with debounce, atomic writes, and bash read ([c9b78c7](https://github.com/Tyka95/kube-kit/commit/c9b78c7c86ce45738eeadf06b219bdc9dc637838))
* trigger homebrew update only on published event ([bc88b3b](https://github.com/Tyka95/kube-kit/commit/bc88b3b343d1b8bfc5ba728113918588bf805fc0))
* **tui:** buffer kubectl output to temp file before less ([e68399b](https://github.com/Tyka95/kube-kit/commit/e68399ba0db4de850ef91d390e069f9feb5020a3))
* **tui:** detect in-update push/pop + load kube ctx + validate AWS on startup ([25f6770](https://github.com/Tyka95/kube-kit/commit/25f677079b2f6a6c5edf6c79f53c510540743a35))
* **tui:** fire Init() on pushed screens; pin footer to bottom; dedup breadcrumb ([bc5d8d7](https://github.com/Tyka95/kube-kit/commit/bc5d8d7453fdba1cf2612ecca2fe3e5403a55f9c))
* **tui:** prefer SSO verification_uri_complete URL so code auto-fills ([4d2ca10](https://github.com/Tyka95/kube-kit/commit/4d2ca10ce96cf92d848dbcddccd2a40e3a90b012))
* **tui:** tea.ExecProcess for sso login + animated spinner during loads ([3257984](https://github.com/Tyka95/kube-kit/commit/325798455994d02ab3c6d6f2315d5b5221b51c36))
* **tui:** visible row selection — brighter bg + integrated left stripe ([b0c614d](https://github.com/Tyka95/kube-kit/commit/b0c614d5229f852e141de5d1dc0905da492008df))
* ui improvements ([4329465](https://github.com/Tyka95/kube-kit/commit/4329465096212057322a8969c77a33793087acd6))
* **ui:** brighten chrome rules so they don't wash out on dark terminals ([962143f](https://github.com/Tyka95/kube-kit/commit/962143f3a664170ca4f877ebc7cac3c4b017ebb9))
* **ui:** stop choose_menu echoing label to stdout ([57e8cd5](https://github.com/Tyka95/kube-kit/commit/57e8cd578ae9cc034a106a7ed19b65c6b5828bbc))
* **ui:** stop choose_menu echoing label to stdout ([e21bb21](https://github.com/Tyka95/kube-kit/commit/e21bb21756b2e84bb0c44a5bbb29fc6310988a4f))
* use PAT for release-please to trigger downstream workflows ([28215df](https://github.com/Tyka95/kube-kit/commit/28215df585d2e78ce09e32695f5af1b55bcf78c2))


### Reverts

* restore original ui.sh, remove broken resize handling ([ba8d9b3](https://github.com/Tyka95/kube-kit/commit/ba8d9b322ee0d74ef556bb6825937dbf65516c93))

## [0.1.13](https://github.com/Tyka95/kube-kit/compare/v0.1.12...v0.1.13) (2026-05-19)


### Features

* Add config/state, paging, UI and UX tweaks ([27d30be](https://github.com/Tyka95/kube-kit/commit/27d30befdbaf0f158c987a0e94868b7cc09cacd2))
* auto-adapt menu to terminal size on resize ([4bfefbf](https://github.com/Tyka95/kube-kit/commit/4bfefbf3d98b459f9b3303d498c6c6dae67a6f70))
* AWS session state, footer status bar, RDS auto-discovery, v0.2 chrome ([#9](https://github.com/Tyka95/kube-kit/issues/9)) ([00cf51b](https://github.com/Tyka95/kube-kit/commit/00cf51ba5be59ad3ca353ed71acd18f8febc9514))
* **go:** port awssession + rds + kctx + config packages ([ae1beb9](https://github.com/Tyka95/kube-kit/commit/ae1beb9b95bcb8c4d90592283292c069108396cf))
* **go:** scaffold Bubble Tea project structure ([ade3ee3](https://github.com/Tyka95/kube-kit/commit/ade3ee32db8a4b7f32a9b5dd73a73570364ed2d7))
* **go:** wire all screens, tunnel + commands packages, help overlay ([dbcfb9f](https://github.com/Tyka95/kube-kit/commit/dbcfb9ff5a0e79169cb6e1b982cfeb89bacedbb0))
* initial release of KubeKit ([1b07392](https://github.com/Tyka95/kube-kit/commit/1b07392392ad1819d32639a83380b79a0f63f0fe))
* **release:** goreleaser pipeline, CI workflow, release-please switch to go module ([a0f00ce](https://github.com/Tyka95/kube-kit/commit/a0f00ce6a102619ca51a6dc23a803051b55f0aa3))
* **theme:** ShimmerGlowAt foreground helper for picker row glow ([f4c0282](https://github.com/Tyka95/kube-kit/commit/f4c028297b92cafcd6dd337679846254397116d6))
* **theme:** switch palette from Tokyo Night to Nord ([8b835fa](https://github.com/Tyka95/kube-kit/commit/8b835fa2c56c40c05a62d08313dd0e064c46b6fb))
* **tui:** auto-copy SSO verification code to clipboard ([71a5780](https://github.com/Tyka95/kube-kit/commit/71a5780f4b7cd5485eecf6904e7333efc000a7b7))
* **tui:** auto-open SSO login URL in the user's default browser ([d68c1eb](https://github.com/Tyka95/kube-kit/commit/d68c1ebac717266ed066ea605fc8595513dc8535))
* **tui:** continuous shimmer on the resting selected row ([c049cd0](https://github.com/Tyka95/kube-kit/commit/c049cd01cda416601b56595de14f9d6b8e0be96f))
* **tui:** embed SSO login flow inside the TUI (no more terminal hand-off) ([df5ce42](https://github.com/Tyka95/kube-kit/commit/df5ce42ce09ea83c6f1e8ce90d8a01c4912c573f))
* **tui:** fix stale AWS header after sso login + visible animations ([69d6ef8](https://github.com/Tyka95/kube-kit/commit/69d6ef80613c7a880febc86db729e7d41592e7d1))
* **tui:** floating '❯' caret marker on selected row + hide terminal cursor ([ae1be45](https://github.com/Tyka95/kube-kit/commit/ae1be450e651fcc4cbed7f75d890e897cb695da7))
* **tui:** pre-flight SSO login screen before suspending ([972dbe5](https://github.com/Tyka95/kube-kit/commit/972dbe58cb5546da424d75617570adb2f9f0a4de))
* **tui:** real pod listing via kubectl ([3308161](https://github.com/Tyka95/kube-kit/commit/33081610dd5e3f74452f2f74f5f2315aba07d6c1))
* **tui:** real Pods + Deployments actions ([8db356b](https://github.com/Tyka95/kube-kit/commit/8db356bc18e621428c13891892a809b9b86a1439))
* **tui:** real Resources actions (Namespaces / Services / Ingress) ([e52518a](https://github.com/Tyka95/kube-kit/commit/e52518aec6ae1a489f6017714ce4b42a896dd561))
* **tui:** route PickerHelpMsg and PickerCommandMsg at the app level ([11dc521](https://github.com/Tyka95/kube-kit/commit/11dc521d95668da0d56ae37fbf0468232bf26dd8))
* **tui:** selection-move flash animation + accent bar + styled callouts ([09e5696](https://github.com/Tyka95/kube-kit/commit/09e56963da59315663f4a1a7992f10f9c2bd8e89))
* **tui:** smooth RGB-interpolated selection fade, drop ugly slide ([619b8b6](https://github.com/Tyka95/kube-kit/commit/619b8b6c3b11b8589b4a6b71ce6e4491bab77c67))


### Bug Fixes

* addional UI improvements and stability ([1e844bf](https://github.com/Tyka95/kube-kit/commit/1e844bf66eae710fe8fd79cbf94d3aece60fb1ef))
* **awssession:** resolve region from cluster URL when context is an alias; rds skip on empty region ([c81b899](https://github.com/Tyka95/kube-kit/commit/c81b8992b9d1a4ccb6bbbc724e7b200712900eaf))
* **aws:** show short cluster name in context picker, not full ARN ([219de9b](https://github.com/Tyka95/kube-kit/commit/219de9b5d05af130e80d4da37401f4703e079152))
* **aws:** show short cluster name in context picker, not full ARN ([c0fd9bf](https://github.com/Tyka95/kube-kit/commit/c0fd9bf56dec5abc6517eb932846f23ac18be1e9))
* full screen clear on terminal resize to prevent corruption ([124339c](https://github.com/Tyka95/kube-kit/commit/124339c5873b820fcd300c00cf692a352d94e0cf))
* improve homebrew tap update trigger ([9f35bf5](https://github.com/Tyka95/kube-kit/commit/9f35bf56ee02a64c9ed61ea2e8681a60e9a6c61c))
* improved aliases ([cffb91a](https://github.com/Tyka95/kube-kit/commit/cffb91afdead140e70be044970b45efed04ec201))
* override all gum pink defaults with cyan theme ([f61974c](https://github.com/Tyka95/kube-kit/commit/f61974c910e72e3ba2e119a60bd26c1b6c81de26))
* **picker:** rune-safe row render with foreground-only shimmer glow ([9265339](https://github.com/Tyka95/kube-kit/commit/9265339be78220b0cebd0d038d536e966aaa8ac6))
* proper alt screen reset and line clearing on terminal resize ([fae7576](https://github.com/Tyka95/kube-kit/commit/fae7576dd25a5b02b353ea1ee215541373fc91d2))
* **release:** create release as draft so assets can upload ([19ba85c](https://github.com/Tyka95/kube-kit/commit/19ba85cabcbb30cda5121309e9db398797b0f395))
* **release:** create release as draft so assets can upload ([2ecb8df](https://github.com/Tyka95/kube-kit/commit/2ecb8df181b24994540800d13d791bb72cf79162))
* rewrite resize handling with debounce, atomic writes, and bash read ([c9b78c7](https://github.com/Tyka95/kube-kit/commit/c9b78c7c86ce45738eeadf06b219bdc9dc637838))
* trigger homebrew update only on published event ([bc88b3b](https://github.com/Tyka95/kube-kit/commit/bc88b3b343d1b8bfc5ba728113918588bf805fc0))
* **tui:** buffer kubectl output to temp file before less ([e68399b](https://github.com/Tyka95/kube-kit/commit/e68399ba0db4de850ef91d390e069f9feb5020a3))
* **tui:** detect in-update push/pop + load kube ctx + validate AWS on startup ([25f6770](https://github.com/Tyka95/kube-kit/commit/25f677079b2f6a6c5edf6c79f53c510540743a35))
* **tui:** fire Init() on pushed screens; pin footer to bottom; dedup breadcrumb ([bc5d8d7](https://github.com/Tyka95/kube-kit/commit/bc5d8d7453fdba1cf2612ecca2fe3e5403a55f9c))
* **tui:** prefer SSO verification_uri_complete URL so code auto-fills ([4d2ca10](https://github.com/Tyka95/kube-kit/commit/4d2ca10ce96cf92d848dbcddccd2a40e3a90b012))
* **tui:** tea.ExecProcess for sso login + animated spinner during loads ([3257984](https://github.com/Tyka95/kube-kit/commit/325798455994d02ab3c6d6f2315d5b5221b51c36))
* **tui:** visible row selection — brighter bg + integrated left stripe ([b0c614d](https://github.com/Tyka95/kube-kit/commit/b0c614d5229f852e141de5d1dc0905da492008df))
* ui improvements ([4329465](https://github.com/Tyka95/kube-kit/commit/4329465096212057322a8969c77a33793087acd6))
* **ui:** brighten chrome rules so they don't wash out on dark terminals ([962143f](https://github.com/Tyka95/kube-kit/commit/962143f3a664170ca4f877ebc7cac3c4b017ebb9))
* **ui:** stop choose_menu echoing label to stdout ([57e8cd5](https://github.com/Tyka95/kube-kit/commit/57e8cd578ae9cc034a106a7ed19b65c6b5828bbc))
* **ui:** stop choose_menu echoing label to stdout ([e21bb21](https://github.com/Tyka95/kube-kit/commit/e21bb21756b2e84bb0c44a5bbb29fc6310988a4f))
* use PAT for release-please to trigger downstream workflows ([28215df](https://github.com/Tyka95/kube-kit/commit/28215df585d2e78ce09e32695f5af1b55bcf78c2))


### Reverts

* restore original ui.sh, remove broken resize handling ([ba8d9b3](https://github.com/Tyka95/kube-kit/commit/ba8d9b322ee0d74ef556bb6825937dbf65516c93))

## [0.1.12](https://github.com/Tyka95/kube-kit/compare/v0.1.11...v0.1.12) (2026-05-19)


### Features

* **go:** port awssession + rds + kctx + config packages ([ae1beb9](https://github.com/Tyka95/kube-kit/commit/ae1beb9b95bcb8c4d90592283292c069108396cf))
* **go:** scaffold Bubble Tea project structure ([ade3ee3](https://github.com/Tyka95/kube-kit/commit/ade3ee32db8a4b7f32a9b5dd73a73570364ed2d7))
* **go:** wire all screens, tunnel + commands packages, help overlay ([dbcfb9f](https://github.com/Tyka95/kube-kit/commit/dbcfb9ff5a0e79169cb6e1b982cfeb89bacedbb0))
* **release:** goreleaser pipeline, CI workflow, release-please switch to go module ([a0f00ce](https://github.com/Tyka95/kube-kit/commit/a0f00ce6a102619ca51a6dc23a803051b55f0aa3))
* **theme:** ShimmerGlowAt foreground helper for picker row glow ([f4c0282](https://github.com/Tyka95/kube-kit/commit/f4c028297b92cafcd6dd337679846254397116d6))
* **theme:** switch palette from Tokyo Night to Nord ([8b835fa](https://github.com/Tyka95/kube-kit/commit/8b835fa2c56c40c05a62d08313dd0e064c46b6fb))
* **tui:** auto-copy SSO verification code to clipboard ([71a5780](https://github.com/Tyka95/kube-kit/commit/71a5780f4b7cd5485eecf6904e7333efc000a7b7))
* **tui:** auto-open SSO login URL in the user's default browser ([d68c1eb](https://github.com/Tyka95/kube-kit/commit/d68c1ebac717266ed066ea605fc8595513dc8535))
* **tui:** continuous shimmer on the resting selected row ([c049cd0](https://github.com/Tyka95/kube-kit/commit/c049cd01cda416601b56595de14f9d6b8e0be96f))
* **tui:** embed SSO login flow inside the TUI (no more terminal hand-off) ([df5ce42](https://github.com/Tyka95/kube-kit/commit/df5ce42ce09ea83c6f1e8ce90d8a01c4912c573f))
* **tui:** fix stale AWS header after sso login + visible animations ([69d6ef8](https://github.com/Tyka95/kube-kit/commit/69d6ef80613c7a880febc86db729e7d41592e7d1))
* **tui:** floating '❯' caret marker on selected row + hide terminal cursor ([ae1be45](https://github.com/Tyka95/kube-kit/commit/ae1be450e651fcc4cbed7f75d890e897cb695da7))
* **tui:** pre-flight SSO login screen before suspending ([972dbe5](https://github.com/Tyka95/kube-kit/commit/972dbe58cb5546da424d75617570adb2f9f0a4de))
* **tui:** real pod listing via kubectl ([3308161](https://github.com/Tyka95/kube-kit/commit/33081610dd5e3f74452f2f74f5f2315aba07d6c1))
* **tui:** real Pods + Deployments actions ([8db356b](https://github.com/Tyka95/kube-kit/commit/8db356bc18e621428c13891892a809b9b86a1439))
* **tui:** real Resources actions (Namespaces / Services / Ingress) ([e52518a](https://github.com/Tyka95/kube-kit/commit/e52518aec6ae1a489f6017714ce4b42a896dd561))
* **tui:** route PickerHelpMsg and PickerCommandMsg at the app level ([11dc521](https://github.com/Tyka95/kube-kit/commit/11dc521d95668da0d56ae37fbf0468232bf26dd8))
* **tui:** selection-move flash animation + accent bar + styled callouts ([09e5696](https://github.com/Tyka95/kube-kit/commit/09e56963da59315663f4a1a7992f10f9c2bd8e89))
* **tui:** smooth RGB-interpolated selection fade, drop ugly slide ([619b8b6](https://github.com/Tyka95/kube-kit/commit/619b8b6c3b11b8589b4a6b71ce6e4491bab77c67))


### Bug Fixes

* **awssession:** resolve region from cluster URL when context is an alias; rds skip on empty region ([c81b899](https://github.com/Tyka95/kube-kit/commit/c81b8992b9d1a4ccb6bbbc724e7b200712900eaf))
* **picker:** rune-safe row render with foreground-only shimmer glow ([9265339](https://github.com/Tyka95/kube-kit/commit/9265339be78220b0cebd0d038d536e966aaa8ac6))
* **tui:** buffer kubectl output to temp file before less ([e68399b](https://github.com/Tyka95/kube-kit/commit/e68399ba0db4de850ef91d390e069f9feb5020a3))
* **tui:** detect in-update push/pop + load kube ctx + validate AWS on startup ([25f6770](https://github.com/Tyka95/kube-kit/commit/25f677079b2f6a6c5edf6c79f53c510540743a35))
* **tui:** fire Init() on pushed screens; pin footer to bottom; dedup breadcrumb ([bc5d8d7](https://github.com/Tyka95/kube-kit/commit/bc5d8d7453fdba1cf2612ecca2fe3e5403a55f9c))
* **tui:** prefer SSO verification_uri_complete URL so code auto-fills ([4d2ca10](https://github.com/Tyka95/kube-kit/commit/4d2ca10ce96cf92d848dbcddccd2a40e3a90b012))
* **tui:** tea.ExecProcess for sso login + animated spinner during loads ([3257984](https://github.com/Tyka95/kube-kit/commit/325798455994d02ab3c6d6f2315d5b5221b51c36))
* **tui:** visible row selection — brighter bg + integrated left stripe ([b0c614d](https://github.com/Tyka95/kube-kit/commit/b0c614d5229f852e141de5d1dc0905da492008df))
* **ui:** brighten chrome rules so they don't wash out on dark terminals ([962143f](https://github.com/Tyka95/kube-kit/commit/962143f3a664170ca4f877ebc7cac3c4b017ebb9))

## [0.1.11](https://github.com/Tyka95/kube-kit/compare/v0.1.10...v0.1.11) (2026-05-13)


### Bug Fixes

* **ui:** stop choose_menu echoing label to stdout ([57e8cd5](https://github.com/Tyka95/kube-kit/commit/57e8cd578ae9cc034a106a7ed19b65c6b5828bbc))
* **ui:** stop choose_menu echoing label to stdout ([e21bb21](https://github.com/Tyka95/kube-kit/commit/e21bb21756b2e84bb0c44a5bbb29fc6310988a4f))

## [0.1.10](https://github.com/Tyka95/kube-kit/compare/v0.1.9...v0.1.10) (2026-05-11)


### Bug Fixes

* **aws:** show short cluster name in context picker, not full ARN ([219de9b](https://github.com/Tyka95/kube-kit/commit/219de9b5d05af130e80d4da37401f4703e079152))
* **aws:** show short cluster name in context picker, not full ARN ([c0fd9bf](https://github.com/Tyka95/kube-kit/commit/c0fd9bf56dec5abc6517eb932846f23ac18be1e9))

## [0.1.9](https://github.com/Tyka95/kube-kit/compare/v0.1.8...v0.1.9) (2026-05-11)


### Features

* AWS session state, footer status bar, RDS auto-discovery, v0.2 chrome ([#9](https://github.com/Tyka95/kube-kit/issues/9)) ([00cf51b](https://github.com/Tyka95/kube-kit/commit/00cf51ba5be59ad3ca353ed71acd18f8febc9514))

## [0.1.8](https://github.com/Tyka95/kube-kit/compare/v0.1.7...v0.1.8) (2026-03-30)


### Bug Fixes

* addional UI improvements and stability ([1e844bf](https://github.com/Tyka95/kube-kit/commit/1e844bf66eae710fe8fd79cbf94d3aece60fb1ef))

## [0.1.7](https://github.com/Tyka95/kube-kit/compare/v0.1.6...v0.1.7) (2026-03-24)


### Bug Fixes

* improved aliases ([cffb91a](https://github.com/Tyka95/kube-kit/commit/cffb91afdead140e70be044970b45efed04ec201))

## [0.1.6](https://github.com/Tyka95/kube-kit/compare/v0.1.5...v0.1.6) (2026-03-24)


### Features

* Add config/state, paging, UI and UX tweaks ([27d30be](https://github.com/Tyka95/kube-kit/commit/27d30befdbaf0f158c987a0e94868b7cc09cacd2))

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
