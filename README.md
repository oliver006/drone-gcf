[![Build Status](https://cloud.drone.io/api/badges/oliver006/drone-gcf/status.svg)](https://cloud.drone.io/oliver006/drone-gcf) [![Coverage Status](https://coveralls.io/repos/github/oliver006/drone-gcf/badge.svg?branch=master)](https://coveralls.io/github/oliver006/drone-gcf?branch=master)

## Drone-GCF - a Drone.io plugin to deploy Google Cloud Functions

![](https://cloud.google.com/_static/images/cloud/products/logos/svg/functions.svg)


### Overview

The plugin supports deploying, listing, calling, and deleting of multiple functions at once.
See below for an example drone.yml configuration that deploys several functions
to GCP Cloud Functions and later deletes two of them.
A service account is needed for authentication to GCP and should be provided
as json string via drone secrets. In the configuration below, the json of
the service account key file is stored in the drone secret called `token`.

#### Deploying Cloud Functions

Example `.drone.yml` file (drone.io 1.0 format):

```yaml
kind: pipeline
name: default

steps:
  - name: test
    image: golang
    commands:
      - "go test -v"
    when:
      event:
        - push


  - name: deploy-cloud-functions
    image: oliver006/drone-gcf
    settings:
      action: deploy
      project: myproject
      token:
        from_secret: token
      runtime: go111
      env_secret_db_password:
        from_secret: db_password_prod
      env_secret_user_api_key:
        from_secret: user_api_key_prod
      functions:
        - TransferFileToGCS:
          - trigger: http
            memory: 2048MB
            allow_unauthenticated: true
            environment:
              - ENV_VAR: env_var_value_123
        - HandleEvents:
          - trigger: topic
            trigger_resource: "projects/myproject/topics/mytopic"
            memory: 512MB
        - ProcessEmails:
          - trigger: http
            memory: 512MB
            runtime: python37
            source: ./python/src/functions/

    when:
      event: push
      branch: master


  - name: delete-cloud-functions
    image: oliver006/drone-gcf
    settings:
      action: delete
      functions:
        - TransferFileToGCS
        - HandleEvents
    when:
      event: tag
```


The plugin supports several types of Google Cloud Function triggers:
- `http`   - triggered for every request to an HTTP endpoint, no other parameters are needed. (see gcloud output for URL of the endpoint).
- `bucket` - triggered for every change in files in a GCS bucket. Supply the name of the bucket via `trigger_resource`.
- `topic`  - triggered for every message published to a PubSub topic. Supply the name of the topic via `trigger_resource`.
- `event`  - triggered for every event of the type specified via `trigger_event` of the resource specified via `trigger_resource`.

See the output of `gcloud functions deploy --help` for more information regarding the setup of triggers.

When deploying a function, there is the option to deploy as a public function. This can be configured by setting `allow_unauthenticated` to `true`. This adds the --allow-unauthenticated flag described [here](https://cloud.google.com/sdk/gcloud/reference/functions/deploy#--allow-unauthenticated) to the deploy command. Please note that this expects a boolean value, either `true` or `false`.

By default, the plugin will use the GCP project ID of the service account provided but you can override it
by setting the `project` parameter.

The runtime can be set on a per-function basis or for all functions at once. In the example above, the runtime
is set to `go111` for all functions and then overwritten with `python37` for just `ProcessEmails`.
This will result in the plugin deploying three functions, two in Golang and one in Python. \
If no runtime setting is provided at all, the plugin will fail.

Similarly, you can set the `source` location of each function in case you keep the code in separate folders.

There are two ways to set environment variables when deploying cloud functions:
- from a secret
- putting the value directly into the drone.yml file

To pull in an environment variable value from a secret, add an entry to the settings that starts
with `env_secret_` followed by the name as which the variable will be made available to the cloud function.
In the config example above, drone will pull the values from the secrets `db_password_prod` and `user_api_key_prod` and
make them available as `DB_PASSWORD` and `USER_API_KEY`. (env variable names will be upper-case) \

Environment variables from secrets will be made available to *all* functions that you deploy within one drone step.
If, for whatever reasons, this is not acceptable and you need to keep them separate then you have to use
multiple drone steps, one for each function.


#### Calling Cloud Functions

You can also trigger a cloud function by using `call` as the action.
Optionally, you can supply a json data string that will be passed to the function.

```yaml
  - name: call-cloud-function
    image: oliver006/drone-gcf
    settings:
      action: call
      project: myproject
      token:
        from_secret: token
      functions:
        - UpdateDatabase:
          - data: '{"key": "value"}'
    when:
      event: tag

```
