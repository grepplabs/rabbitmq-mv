# rabbitmq-mv

rabbitmq-mv is a utility to move messages from one rabbitmq queue to another one.


## Build binary

   Linux

        make build.linux

   MacOS

        make build.darwin

   Windows

        make build.windows
    

### Help output

    Usage of ./rabbitmq-mv:
      -dst-queue string
            Destination queue name
      -dst-uri string
            Destination URI e.g. amqp://username:password@rabbitmq-fqdn:5672
      -limit int
            Limit the number of messages
      -src-queue string
            Source queue name
      -src-uri string
            Source URI e.g. amqp://username:password@rabbitmq-fqdn:5672
      -tx
            Use producer transactions (slow)




## Usage examples

```
rabbitmq-mv -limit 1 -from-error-queue -dst-queue test-queue-1 -dst-uri amqp://username:password@rabbitmq-fqdn:5672
rabbitmq-mv -src-queue test-queue-2 -dst-queue test-queue-1 -dst-uri amqp://username:password@rabbitmq-fqdn:5672
rabbitmq-mv -src-queue test-queue-2 -src-uri amqp://username:password@rabbitmq-fqdn:5672 -dst-queue test-queue-1 -dst-uri amqp://username:password@rabbitmq-fqdn:5672
```

