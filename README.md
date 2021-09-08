# bridge-k8s-test

Trying to prototype test and see if it works

To repeat the test change the host `test-bridge-k8s-20-53-124-141.nip.io` in the ingress yaml file to match with your external IP of your ingress controller (replace the `20-53-124-141` with yours) . Then you can start the test.

```
## Check out the repo
alias k=kubectl
k create namespace test-k8sbridge
cd golang/k8s-deployment/
k config set-context --current --namespace test-k8sbridge
cat *.yaml | k apply -f-
# If you found errors then run one by one, not sure why
for fn in configmap.yaml deployment.yaml service.yaml ingress.yaml; do
    k apply -f $fn
done
```

Now start vscode, and open the root folder.
Then start the bridge k8s procedure - see https://docs.microsoft.com/en-us/visualstudio/containers/bridge-to-kubernetes?view=vs-2019 for details.

At the end you should have two external endpoint, one like this `https://test-bridge-k8s-<your ip>.nip.io` will reach your remote deployment, and the other `https://<your-local-username>-<XXX>.test-bridge-k8s-<your ip>.nip.io` will reach your local work station running the vscode. Make a request to each endpoint and see the count in the local vscode console (or not).

The `XXX` is configured by the vscode plugin, you can see it in the json config or do a `k get ingress` and watch the new ingress it created like `test-bridge-k8s-stevek-test-bridge-k8s-webservic-cloned-routing` the value of `HOSTS` field.

Try to save some log into the local server for example

```
curl -k -X POST -H "X-Webserver-Template-Token: changeme" -H "Content-Type: application/x-www-form-urlencoded" 'https://<your-local-username>-<XXX>.test-bridge-k8s-<your ip>.nip.io/savelog' --data-urlencode 'logfile={"event": "started", "file": "codeception.yml", "error_code": -1}' --data-urlencode "message='test'" --data-urlencode "application='test app'"
```

Try to run a select

```
curl -k -X POST -H 'Accept: application/json' -H "X-Webserver-Template-Token: changeme" -H "X-Gitlab-Event: Deployment Hook" 'https://<your-local-username>-<XXX>.test-bridge-k8s-<your ip>.nip.io/sql' -d 'sql=select * from log'
```

Try to fault it by change the table name from the above - from `from log` to `from log1` and see the errors in vscode. Or put a break point in vs code.

# Status
As of now, branch `main` contains the prototype works for both cased, `deployment` in the folder `k8s-deployment` and `statefulset` in `k8s-statefullset`
