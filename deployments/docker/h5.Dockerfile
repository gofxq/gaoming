FROM node:24-alpine

WORKDIR /app

COPY h5/package.json h5/yarn.lock ./
RUN yarn install --frozen-lockfile

COPY h5/ ./

EXPOSE 5174

CMD ["yarn", "dev"]
