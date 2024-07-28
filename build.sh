VERSION="1.0.1"
docker build --platform linux/amd64 -t gcr.io/personal-site-353523/shoot:$VERSION .
docker push gcr.io/personal-site-353523/shoot:$VERSION
