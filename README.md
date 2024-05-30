## Setup
>**NOTE:**  The README assumes an environment where the code is cloned, edited, and built locally, and is deployed on a remote cloudlab cluster. Adjust the commands as necessary depending on the actual execution deployment environment. 

### Cloning Repositories
Clone the following repositories. Refer to [this section below](#-copying-files-to-each-node) for a description on different cloning options
```
git clone git@github.com:gt-epl/feo.git
git clone git@github.com:gt-epl/feo-loadgen.git
git clone git@github.com:gt-epl/feo-notebooks.git
```


### Modifying Configurations
Append the following entry to a sshconfig file (e.g. `~/.ssh/config`)
The following is an example configuration. Note the alias `clabcl0` defined next to `Host`. This alias is an important key in `loadgen/run_load.py`. 
For the scripts to work correctly, the last host should be `clabsvr`. You may adjust the scripts if you wish to use a different nameset. 

```
Host clabcl0
  User {your cloudlab username}
  Hostname {your cloudlab hostname}
  Port {your cloudlab portnumber}
  IdentityFile {path to the private SSH key registered in cloudlab}
  StrictHostKeyChecking=no
  UserKnownHostsFile ~/.ssh/clab_hosts
  ServerAliveInterval 120

Host clabcl1
...

Host clabsvr
...
```

On the `loadgen/run_load.py` file, adjust the configurations in the commented area.

On `feo/config.template.yml`, adjust the peer address & controller address so that the nodes can send messages to each other. Make sure that the order of the peer address matches the order of the nodes in 'hosts'.


On each profile file under `loadgen/profiles`, the hostname in the first column should match the names in the list `hosts` defined in `run_load.py`.

### Copying files to each node
There are 3 options to run the files
1) Run `loadgen/run_load.py` (and eventually) `./utils/sync.sh` locally. In this case you do not have to copy an sshconfig file or a sshkey file to a remote machine. `loaden/run_load.py` takes care of copying any binary and/or application, util files. 
>**Note:** In `run_load.py`, `CONFIG_EXEC_LOCAL` must be set to `True`. Also, the local node must have go & python installed to build/run the scripts.

2) Copy this directory (`feo`) into `clabsvr` node. We run `loadgen/run_load.py` *locally*, but the script will run `utils/sync.sh` *remotely*.
>**Note:** This requires us to copy a SSH private key to `clabsvr`, so that the `clabsvr` node can `ssh` into other nodes. Create a dedicated, NON_CRITICAL SSH key for this instance!

```
cd ../
rsync {path to the sshconfig file defined above} clabsvr:~/.ssh/{path to sshconfig file}
rsync {path to the ssh identity file (privatekey) defined above} clabsvr:~/.ssh/{path to identity file}
rsync -avz ./feo clabsvr:~/  OR  ssh clabsvr; cd ~; git clone git@github.gatech.edu:faasedge/feo.git
```

3) Copy both `../feo` and `../loadgen` to `clabsvr` node. This allows us to avoid the need to setup a build environment locally. 
>**Note:** In `run_load.py`, `CONFIG_EXEC_LOCAL` must be set to `True`. Also, the results residing in `clabsvr:RESULT_DIR` must be moved to a persistent storage. 

In addition to the above command, run:
```
rsync -avz ./loadgen clabsvr:~/  OR  ssh clabsvr; cd ~; git clone git@github.gatech.edu:faasedge/loadgen.git
```


## Run evaluations 
```
python run_load.py profile.csv
```

If `ACTUAL` and `FETCH_RESULTS` are set to `True` in run_load.py, the results will appear in `RESDIR`, also defined in run_load.py. 

Use the notebooks in `feo-notebooks` to plot the results. 

## How to check if deployment is correct?
On each node, 
- To check if openwhisk has been deployed
```
docker ps
```

- To check if an action has been deployed
```
bash list.sh http://localhost:3233
```

- To check if an action is correctly invoked
```
bash apps/copy/invoke_action.sh http://localhost:3233
```

- To check if the inter-node latency has successfully been set using `set_latency.sh`, simply use `ping`
