rsync -auzv /Volumes/afs/dev/github.com/gofxq/gaoming/* sh:/home/u/dev/gaoming
ssh sh "cd /home/u/dev/gaoming && make docker-up"