# website/Dockerfile
FROM node:18-alpine

WORKDIR /app
COPY package.json /app/
COPY package-lock.json /app/  # если есть
RUN npm install

COPY server.js /app/
COPY ../config.yaml /app/config.yaml
COPY public /app/public

CMD ["npm", "start"]
