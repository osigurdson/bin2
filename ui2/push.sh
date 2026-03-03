podman login https://lax.vultrcr.com/bin2 -u $VULTR_CR_USERNAME -p $VULTR_CR_PASSWORD
mv .env.local __env_local
npm run build
podman build -t ui:$1 .
podman tag ui:$1 lax.vultrcr.com/bin2/ui:$1
podman push lax.vultrcr.com/bin2/ui:$1
mv __env_local .env.local
