FROM node:23-alpine AS build

ARG VITE_API_BASE_URL=http://api:8080/api/v1
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

WORKDIR /app
COPY web/package*.json ./
RUN npm install
COPY web ./
RUN npm run build

FROM node:23-alpine
WORKDIR /app
COPY --from=build /app/package*.json ./
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
EXPOSE 4173
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "4173"]
