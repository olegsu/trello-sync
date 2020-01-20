echo "Creating secret in kuberentes cluster"
echo $TRELLO_BOARD_ID > ./trello-board-id
echo $TRELLO_TOKEN > ./trello-token
echo $TRELLO_APP_KEY > ./trello-app-key
echo $GOOGLE_SPREADSHEET_ID > ./google-spreadsheet-id
cat $GOOGLE_SERVICE_ACCOUNT_PATH | base64 > ./google-service-account-b64

kubectl create secret generic trello-sync \
    --from-file=./trello-board-id \
    --from-file=./trello-token \
    --from-file=./trello-app-key \
    --from-file=./google-spreadsheet-id \
    --from-file=./google-service-account-b64 
