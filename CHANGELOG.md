<!-- SPDX-FileCopyrightText: 2026 The SonicRed contributors.
     SPDX-License-Identifier: MPL-2.0
-->

Release 1.11.3
==============

- added Go runtime metrics (OpenTelemetry runtime instrumentation)
- fixed service references between the helm monitoring subcharts
- fixed log and trace forwarding from Alloy to VictoriaLogs and VictoriaTraces
- disabled the unintentionally deployed Vector DaemonSet in the helm chart
- grafana now uses the VictoriaMetrics datasource and ships a Go runtime
  dashboard
- the helm chart now requires a Grafana admin password if monitoring is enabled
- fixed VictoriaLogs and VictoriaTraces not starting due to a too short retention
  period in the helm chart
- the helm chart now sets the pod name as a service instance in the OpenTelemetry
  resource attributes
- the directory listing no longer redirects to add the `lang` parameter, the
  language is selected directly from the `lang` parameter or the
  `Accept-Language` header
- language tags are matched case-insensitively, unknown variants and scripts
  fall back step by step (e.g. `zh-Hant-TW` to `zh-Hant`), languages with
  `q=0` are no longer selected
- dates and sorting in the directory listing use the language of the listing
- fixed links in the directory listing for names containing `?`, `#` or `%`
- fixed the Serbian and Kurdish translations, Sorani moved to `ckb`, added
  `zh-Hant` and `pa-Arab`
- invalid client CA files now produce an error instead of an empty certificate
  pool
- automatic certificate retrieval via ACME now also requires TLS 1.3
- fixed automatic certificate retrieval failing in combination with client
  certificate authentication, the ACME challenge connections no longer require
  a client certificate
- documented that automatic certificate retrieval requires port 443 to be
  reachable
- the certificate cache now defaults to `sonicred/autocert` in the user cache
  directory instead of the temporary directory
- the certificate cache directory must not be inside the web root directory
- the default certificate cache directory must be owned by and only be
  accessible by the current user
- `latest` docker image tag is no longer moved by prereleases, promoting a
  prerelease to a release now triggers the release build
- release builds no longer use the GitHub Actions caches
- re-runs of the release pipeline no longer attest the provenance of a previous
  run
- the helm chart uses the fixed default ports 8080 and 8081, the values
  `service.port` and `service.metricsPort` were removed
- fixed the Alloy service name in the helm chart if the release name contains
  `alloy` or the Alloy chart name is overridden
- Alloy runs as non-root with a restrictive security context in the helm chart
- the Alloy network policy only allows the OTLP ports
- the helm test pod runs with a restrictive security context
- the sources allowed to reach Grafana are configurable via
  `monitoring.networkPolicy.grafana.from`

Release 1.11.2
==============

- updated readme viewer for docker image
- updated docker build-stage image
- updated helm dependencies
- changed to self-maintained GoReportCard
- dependency updates

Release 1.11.1
==============

- updated the minimum required Go version to 1.27
- added more translations to the directory listing
- integrated SBOM generation
- dependency updates

Release 1.11.0
==============

- renaming from SonicWeb to SonicRed to avoid confusion with other preexisting
  projects
- further expanded directory listing translations
- dependency updates

Release 1.10.0
==============

- added more languages to the directory listing translations
- language variant support in directory listing translations
- Makefile determines man-pages automatically
- upgraded Helm dependencies

Release 1.9.0
=============

- extend directory listing with accessibility, localized time format, sorting,
  no-script support and some optical enhancements
- introduced `osv-scanner.toml` to silence non-affecting vulnerability reports
- dependency updates

Release 1.8.1
=============

- hardened workflows
- improved documentation
- fixed an issue in the directory listing concerning symlinks
- included third-party licenses in deliverables
- moved testreport functionality from workflow to Makefile
- dependency updates
- initial MSIX package support

Release 1.8.0
=============

- introduced new `-index` option to control directory listing
- update Makefile to give Version v0.0 in case of no version tag
- dependency updates

Release 1.7.2
=============

- Makefile cleanup and modularization
- default executable prefix changed from `sonic` to `sonicweb`
- TLS test-keys and -certificates now use ECDSA instead of RSA
- dependency updates

Release 1.7.1
=============

- add warning if try-file patterns resemble absolute Windows/UNC paths
  (helps detect MSYS/MinGW auto path conversion)
- dependency updates

Release 1.7.0
=============

- added monitoring to deployment configuration
- reworked Open Telemetry integration
- moved instrumentation and server lifecycle into submodules
- update dependencies
- the minimum Go version is now 1.26

Release 1.6.1
=============

- added asset and container provenance
- added SRI hashes to the documentation demo page
- leaner script loading for the documentation demo page
- dependency updates

Release 1.6.0
=============

- provide prebuild (multi-arch) docker images
- update documentation for docker usage
- check that the build target for cross-compilation is set correctly

Release 1.5.3
=============

- added fuzz testing
- document internal functions
- dependency updates

Release 1.5.2
=============

- changed release artifacts for Windows to 7z
- fixed Windows executable file suffix

Release 1.5.1
=============

- added golangci-lint and adapted code
- dependency updates
- update to Go 1.25
- added Community Code of Conduct
- removed automaxprocs, as this is working fine as of Go 1.25

Release 1.5.0
=============

- added `-clientca` option to enable mTLS connections
- improved shutdown handling
- dependency updates

Release 1.4.1
=============

- update dependencies
- build using Go 1.24.3 that also fixes a vulnerability in os.Root

Release 1.4.0
=============

- added `-tryfile` option, to replace original URI with a given list of
  files to try first
- added a single-page application example to illustrate the try-file usage
- added `-tlscert` and `-tlskey` parameter, enabling serving via TLS
- fixed manpage headers
- add support for automatic certificates via Let's Encrypt
  via `-acmedomain`, `-certcache` and `-acmeendpoint` parameters

Release 1.3.0
=============

- additional-header parameter `-header`
- additional-header-from-file parameter `-headerFile`
- parameter `-wafcfg` to add configurations to the Coraza Web Application Firewall

Release 1.2.1
=============

- use AlphaOne1/geany for logo output
- dependency updates
- source location logging only for DEBUG log level
- removed unused utility functions

Release 1.2.0
=============

- update to Go 1.24
- use os.Root for file access protection
- update dependencies

Release 1.1.0
=============

- add Dockerfile
- added packaging for deb and rpm
- add helm chart
- add opentelemetry tracing support
- added English, German and Spanish man-pages

Release 1.0.0
=============

Initial release

- command line configurable webserver
- access logging
- correlation id
- Coraza WAF middleware
