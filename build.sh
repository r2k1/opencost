docker buildx build --platform=linux/amd64 -t akscm.azurecr.io/opencost:test .
docker push akscm.azurecr.io/opencost:test
kubectl apply -f deployment.yaml
kubectl -n opencost rollout restart deployment/opencost
kubectl -n opencost logs -f deployment/opencost opencost
