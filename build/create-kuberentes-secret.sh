echo "Creating secret in kuberentes cluster"

kubectl create secret generic trello-sync \
    --from-literal=trello-board-id=$TRELLO_BOARD_ID \
    --from-literal=trello-token=$TRELLO_TOKEN \
    --from-literal=trello-app-key=$TRELLO_APP_KEY \
    --from-literal=google-spreadsheet-id=$GOOGLE_SPREADSHEET_ID \
    --from-literal=google-service-account-b64=$(cat $GOOGLE_SERVICE_ACCOUNT_PATH | base64) \

