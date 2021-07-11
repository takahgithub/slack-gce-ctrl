# これは何か
* SlackからGCPのCompute Engineの起動・終了を制御するためのアプリ。
* Google App Engineにデプロイする。
# 実行環境
* Google App Engine
# 開発環境
* mac(macOS Big Sur 11.4)
* go version go1.16.3 darwin/amd64
* Google Cloud SDK 337.0.0  
app-engine-go 1.9.71  
app-engine-python 1.9.91  
bq 2.0.67  
cloud-datastore-emulator 2.1.0  
core 2021.04.16  
gsutil 4.61
# 開発環境構築
* これに従ってやったはず
https://cloud.google.com/appengine/docs/standard/go/building-app?hl=ja
# デプロイ方法
* gcloud app deploy
# 参考にしたサイト
GCP公式の他、以下のサイトを参考にしました。
https://mpiyok.hatenablog.com/entry/2017/12/10/094638