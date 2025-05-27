curl -i -X POST \
  --url https://chat-ai.academiccloud.de/v1/completions \
  --header 'Accept: application/json' \
  --header 'Authorization: Bearer <api_key>' \
  --header 'Content-Type: application/json'\
  --data '{
  "model": "meta-llama-3.1-8b-instruct",
  "messages":[{"role":"system","content":"You are an assistant."},{"role":"user","content":"What is the weather today?"}],
  "max_tokens": 7,
  "temperature": 0.5,
  "top_p": 0.5
}'
