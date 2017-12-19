#!/usr/bin/bash

BASE=`dirname $BASH_SOURCE`

cd $BASE/data

rm -Rf node_modules dist

npm install

API_BASE="/api/1.0" npm run build-prod

cd ..

go-bindata-assetfs -pkg client -prefix data/dist data/dist/...
