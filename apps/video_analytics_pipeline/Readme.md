# Video Analytics Pipeline

``` 
                                            +----------+  
                                      ,-----| Annotate |  
 o       +--------+     +--------+    |     +----------+
/|\ -----| Filter |-----| Detect |----+
/ \      +--------+     +--------+    |     +----------+  
                                      `-----|   Sink   |
                                            +----------+  
```

## Testing Locally
1. Create venv  `python -m venv localenv`
2. Activate     `source localenv/bin/activate`
3. Install Deps `pip install -r requirements.txt`
4. Extract data `tar -xzf va-imgs.tgz`. Get va-imgs.tgz from cosmos:cosmos-str/dataset/feo/datasets/
5. Setup cfg    `sudo ln -sf ./detect/cfg ./cfg` (This is necessary for the local pipeline execution)
6. Run pipeline `python run-pipeline-local.py`

## Testing on openwhisk
Standalone Openwhisk does not have sufficient default memory to instantiate container. Please add memory limts to `whisk` config as follows:
```yaml
  container-pool {
    user-memory: 4096 m
  }
  memory {
    min = 128 m
    max = 2048 m
    std = 256 m
  }
```

1. Deploy all functions: `bash create_action.sh`
2. Run pipeline `python run-pipeline.py`

## Additional Info
Latest docker image is housed at asarma31/openwhisk-video-analytics-pipeline-base on docker hub



