#!/bin/bash
# Push to GitHub — запусти когда будет токен
# 
# 1. Создай токен: github.com → Settings → Developer settings → 
#    Personal access tokens → Fine-grained → Generate new
# 2. Запусти: ./scripts/push-github.sh ghp_ВАШ_ТОКЕН

TOKEN=${1:?"Usage: $0 <github_token>"}

# Create repo via GitHub API
curl -s -H "Authorization: token $TOKEN" \
  https://api.github.com/user/repos \
  -d '{"name":"unkillable-messenger","description":"Decentralized messenger + VPN + content platform","private":false}' \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('html_url', d.get('message','ERROR')))"

# Add remote and push
git remote add origin https://${TOKEN}@github.com/$(git config user.login 2>/dev/null || echo 'USER')/unkillable-messenger.git 2>/dev/null || true
git push -u origin main

echo "Done!"
