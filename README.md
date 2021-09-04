# Outdated instruction

I have arranged the dirs a bit so these info below just for information purposes only, I have not had time to update it yet.


# bridge-k8s-test


Trying to prototype test and see if it works

To repeat the test change the host `test-bridge-k8s-52-147-15-249.nip.io` in the ingress yaml file to match with your external IP of your ingress controller (replace the `52-147-15-249` with yours) . Then you can start the test.

```
alias k=kubectl
k create namespace test-k8sbridge
cd golang/nginx-ingress-statefullset/k8s/
k config set-context --current --namespace test-k8sbridge
cat *.yaml | k apply -f-
```

Now start vscode, and open the root folder.
Then start the bridge k8s procedure - see https://docs.microsoft.com/en-us/visualstudio/containers/bridge-to-kubernetes?view=vs-2019 for details.

At the end you should have two external endpoint, one like this `https://test-bridge-k8s-<your ip>.nip.io` will reach your remote deployment, and the other `https://<your-local-username>xxx.test-bridge-k8s-<your ip>.nip.io` will reach your local work station running the vscode. Make a request to each endpoint and see the count in the local vscode console (or not).

The branch `main` is the branch having the issues.
The branch `working` does work.

I can create more test scenarios later on.
