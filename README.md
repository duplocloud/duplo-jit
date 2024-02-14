# duplo-jit
Command-line tools for JIT Duplo, AWS and Kubernetes access

## Installation

### From release zip files

See the *Releases* section of this repository.

- Download a release artifact that matches your system's architecture.
- Unzip the artifact.
- Install the binaries somewhere in your `PATH`, such as the `/usr/local/bin` directory.

### With Homebrew

run `brew install duplocloud/tap/duplo-jit` from your terminal

## Usage

### duplo-jit aws

This tool is intended to be used in your `~/.aws/config`.  It provides just-in-time access to AWS using short-lived, temporary access keys.

Example `~/.aws/config` for admin access to Duplo:

```
[profile myduplo-admin]
region=us-west-2
credential_process=duplo-jit aws --admin --host https://MY-DUPLO-HOSTNAME.duplocloud.net --interactive
```

Example `~/.aws/config` for tenant-level access to Duplo:

```
[profile myduplo-tenant]
region=us-west-2
credential_process=duplo-jit aws --tenant MY-TENANT-NAME --host https://MY-DUPLO-HOSTNAME.duplocloud.net --interactive
```

## Command help

### duplo-jit aws --help

```
Usage of duplo-jit:
  -admin
    	Get admin credentials
  -debug
    	Turn on verbose (debugging) output
  -duplo-ops
    	Get Duplo operations credentials
  -host string
    	Duplo API base URL
  -interactive
    	Allow getting Duplo credentials via an interactive browser session
  -no-cache
    	Disable caching (not recommended)
  -tenant string
    	Get credentials for the given tenant
  -token string
    	Duplo API token
  -version
    	Output version information and exit
```

### duplo-jit duplo --help

```
Usage of duplo-jit:
  -debug
    	Turn on verbose (debugging) output
  -host string
    	Duplo API base URL
  -interactive
    	Allow getting Duplo credentials via an interactive browser session
  -no-cache
    	Disable caching (not recommended)
  -token string
    	Duplo API token
  -version
    	Output version information and exit
```

### duplo-jit k8s --help

```
Usage of duplo-jit:
  -debug
    	Turn on verbose (debugging) output
  -host string
    	Duplo API base URL
  -interactive
    	Allow getting Duplo credentials via an interactive browser session
  -no-cache
    	Disable caching (not recommended)
  -plan string
    	Get credentials for the given plan
  -tenant string
    	Get credentials for the given tenant
  -token string
    	Duplo API token
  -version
    	Output version information and exit
```

## Deprecated: duplo-aws-credential-process --help

```
Usage of duplo-aws-credential-process:
  -admin
    	Get admin credentials
  -debug
    	Turn on verbose (debugging) output
  -duplo-ops
    	Get Duplo operations credentials
  -host string
    	Duplo API base URL
  -interactive
    	Allow getting Duplo credentials via an interactive browser session
  -no-cache
    	Disable caching (not recommended)
  -tenant string
    	Get credentials for the given tenant
  -token string
    	Duplo API token
  -version
    	Output version information and exit
```
