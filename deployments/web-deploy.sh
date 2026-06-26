cd web && yarn build && cd ..;

rsync -auzv /Volumes/afs/dev/github.com/gofxq/gaoming/web/dist/* sh:/home/u/run/container/caddy/static/gaoming