# Install Sourcegraph with Docker Compose on Google Cloud

This tutorial shows you how to deploy Sourcegraph via [Docker Compose](https://docs.docker.com/compose/) to a single node running on Google Cloud.

> NOTE: Trying to decide how to deploy Sourcegraph? See [our recommendations](../index.md) for how to choose a deployment type that suits your needs.

---
## Determine server and service requirements 

Use the [resource estimator](../resource_estimator.md) to determine the resource requirements for your environment. You will use this information to set up the instance and configure the docker-compose YAML file. 

## Prepare a fork 

We strongly recommend that you create and run Sourcegraph from your own fork of the reference repository. You will make changes to the default configuration, for example to the docker-compose YAML file, in your fork. The fork will also enable you to keep track of your customizations when upgrading your fork from the reference repo. Refer to the following steps for preparing a clone, which use GitHub as an example, then return to this page:

1. [Fork the reference repo](index.md#fork-the-sourcegraph-reference-repository)
2. [Clone your fork](index.md#clone-your-fork)
3. [Configure a release branch](index.md#configure-a-release-branch)
4. [Configure the YAML file](index.md#configure-the-yaml-file)
5. [Publish changes to your branch](index.md#publish-your-changes-to-the-release-branch)

## Deploy to Google Cloud VM

* [Open your Google Cloud console](https://console.cloud.google.com/compute/instances) to create a new VM instance and click **Create Instance**
* Choose an appropriate machine type (use the [resource estimator](../resource_estimator.md) to find a good starting point for your deployment).
* Under the "Boot Disk" options, select the following:
  * **Operating System**: Ubuntu
  * **Version**: Ubuntu 18.04 LTS
  * **Boot disk type**: SSD persistent disk
* Check the boxes for **Allow HTTP traffic** and **Allow HTTPS traffic** in the **Firewall** section
* Open the **Management, disks, networking, and SSH keys** dropdown section
* Under the **Management** section, add the following in the **Startup script** field:

> NOTE: replace the following variables in the script based on how you created your fork and release branch
>
> `DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL`: Your fork's git clone URL
>
> `DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION`: The git revision containing your fork's customizations to the base Sourcegraph Docker Compose YAML. In the [example](index.md#configure-a-release-branch) the revision is the `release` branch. 

```bash
#!/usr/bin/env bash

set -euxo pipefail

PERSISTENT_DISK_DEVICE_NAME='/dev/sdb'
DOCKER_DATA_ROOT='/mnt/docker-data'

DOCKER_COMPOSE_VERSION='1.29.2'
DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT='/root/deploy-sourcegraph-docker'

# 🚨 Update these variables with the correct values from your fork!
DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL='https://github.com/sourcegraph/deploy-sourcegraph-docker.git'
DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION='v3.42.1'

# Install git
sudo apt-get update -y
sudo apt-get install -y git

# Clone Docker Compose definition
git clone "${DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL}" "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"
cd "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"
git checkout "${DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION}"

# Format (if necessary) and mount GCP persistent disk
device_fs=$(sudo lsblk "${PERSISTENT_DISK_DEVICE_NAME}" --noheadings --output fsType)
if [ "${device_fs}" == "" ] ## only format the volume if it isn't already formatted
then
    sudo mkfs.ext4 -m 0 -E lazy_itable_init=0,lazy_journal_init=0,discard "${PERSISTENT_DISK_DEVICE_NAME}"
fi
sudo mkdir -p "${DOCKER_DATA_ROOT}"
sudo mount -o discard,defaults "${PERSISTENT_DISK_DEVICE_NAME}" "${DOCKER_DATA_ROOT}"

# Mount GCP disk on reboots
DISK_UUID=$(sudo blkid -s UUID -o value "${PERSISTENT_DISK_DEVICE_NAME}")
sudo echo "UUID=${DISK_UUID}  ${DOCKER_DATA_ROOT}  ext4  discard,defaults,nofail  0  2" >> '/etc/fstab'
umount "${DOCKER_DATA_ROOT}"
mount -a

# Install Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-get update -y
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update -y
apt-cache policy docker-ce
apt-get install -y docker-ce docker-ce-cli containerd.io

# Install jq for scripting
sudo apt-get update -y
sudo apt-get install -y jq

# Edit Docker storage directory to mounted volume
DOCKER_DAEMON_CONFIG_FILE='/etc/docker/daemon.json'

## initialize the config file with empty json if it doesn't exist
if [ ! -f "${DOCKER_DAEMON_CONFIG_FILE}" ]
then
    mkdir -p $(dirname "${DOCKER_DAEMON_CONFIG_FILE}")
    echo '{}' > "${DOCKER_DAEMON_CONFIG_FILE}"
fi

## update Docker's 'data-root' to point to our mounted disk
tmp_config=$(mktemp)
trap "rm -f ${tmp_config}" EXIT
sudo cat "${DOCKER_DAEMON_CONFIG_FILE}" | sudo jq --arg DATA_ROOT "${DOCKER_DATA_ROOT}" '.["data-root"]=$DATA_ROOT' > "${tmp_config}"
sudo cat "${tmp_config}" > "${DOCKER_DAEMON_CONFIG_FILE}"

## finally, restart Docker daemon to pick up our changes
sudo systemctl restart --now docker

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
curl -L "https://raw.githubusercontent.com/docker/compose/${DOCKER_COMPOSE_VERSION}/contrib/completion/bash/docker-compose" -o /etc/bash_completion.d/docker-compose

# Run Sourcegraph. Restart the containers upon reboot.
cd "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"/docker-compose
docker-compose up -d
```

* Under the **Disks** section, click **Add new disk**  and add a disk (for storing Docker data) with the following settings:
  * **Type**: SSD Persistent Disk
  * **Description**: "Disk for storing Docker data for Sourcegraph" (or something similarly descriptive)
  * **(optional, recommended) Snapshot schedule**: The most straightfoward way of automatically backing Sourcegraph's data is to set up a [snapshot schedule](https://cloud.google.com/compute/docs/disks/scheduled-snapshots) for this disk. We strongly recommend that you take the time to do so here.
  * **Mode**: Read/write
  * **Deletion rule**: Keep disk
  * **Size**: `250` GB minimum *(As a rule of thumb, Sourcegraph needs at least as much space as all your repositories combined take up. Allocating as much disk space as you can upfront helps you avoid [resizing this disk](https://cloud.google.com/compute/docs/disks/add-persistent-disk#resize_pd) later on.)*

* Create your VM, then navigate to its public IP address.
* If you have configured a DNS entry for the IP, configure `externalURL` to reflect that.
* You may have to wait a minute or two for the instance to finish initializing before Sourcegraph becomes accessible. You can monitor the status by SSHing into the instance and running the following diagnostic commands:

```bash
# Follow the status of the user data script you provided earlier
tail -c +0 -f /var/log/syslog | grep startup-script

# (Once the user data script completes) monitor the health of the "sourcegraph-frontend" container
docker ps --filter="name=sourcegraph-frontend-0"
```

---

## Update your Sourcegraph version

Refer to the [Docker Compose upgrade docs](upgrade.md).

## Storage and Backups

The [Sourcegraph Docker Compose definition](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) uses [Docker volumes](https://docs.docker.com/storage/volumes/) to store its data. The script above [configures Docker](https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file) to store all Docker data on the additional persistent disk that was attached to the instance (mounted at `/mnt/docker-data` - the volumes themselves are stored under `/mnt/docker-data/volumes`) There are a few different ways to backup this data:

* (**recommended**) The most straightfoward method to backup this data is to [snapshot the entire `/mnt/docker-data` persistsent disk](https://cloud.google.com/compute/docs/disks/create-snapshots) on an [automatic, scheduled basis](https://cloud.google.com/compute/docs/disks/scheduled-snapshots). The directions above tell you how to set up this schedule when the instance is first created, but you can also [create the schedule afterwards](https://cloud.google.com/compute/docs/disks/scheduled-snapshots).

* Using an external Postgres instance (see below) lets a service such as [Cloud SQL for PostgreSQL](https://cloud.google.com/sql/docs/postgres/) take care of backing up all of Sourcegraph's user data for you. If the VM instance running Sourcegraph ever dies or is destroyed, creating a fresh instance that's connected to that external Postgres will leave Sourcegraph in the same state that it was before.
