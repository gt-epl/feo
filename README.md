## Setup
>**NOTE:**  The README assumes an environment where the code is cloned, edited, and built locally, and is deployed on a remote cloudlab cluster. Adjust the commands as necessary depending on the actual execution deployment environment. 

### Cloning Repositories
Clone the following repositories: 
```
git clone git@github.gatech.edu:faasedge/feo.git
git clone git@github.gatech.edu:faasedge/loadgen.git
git clone git@github.gatech.edu:faasedge/feo-notebooks.git
```


### Modifying Configurations
Append the following entry to a sshconfig file (e.g. `~/.ssh/config`)
The following is an example configuration. Note the alias `clabcl0` defined next to `Host`. This alias is an important key in `loadgen/run_load.py`. 

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
```

On the `loadgen/run_load.py` file, adjust the configurations in the commented area.

On `feo/config.template.yml`, adjust the peer address & controller address so that the nodes can send messages to each other. Make sure that the order of the peer address matches the order of the nodes in 'hosts'.


On each profile file under `loadgen/profiles`, the hostname in the first column should match the names in the list `hosts` defined in `run_load.py`.

### Copying files to each node
The script `loadgen/run_load.py` will take care of copying relevant binaries, scripts, and application code to each node.

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
