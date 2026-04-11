FROM node:22-alpine

WORKDIR /app

COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile

COPY web/ ./

EXPOSE 5173

CMD ["yarn", "dev", "--host", "0.0.0.0", "--port", "5173", "--strictPort"]
