FROM node:lts

EXPOSE 3000


COPY . /home/node/app

WORKDIR /home/node/app

RUN npm ci

USER node

CMD npm start