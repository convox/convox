FROM drupal:10.2.8

RUN apt-get update &&apt-get install wget lsb-release gnupg sudo -y

RUN apt install software-properties-common python3-pip python3-launchpadlib -y 

RUN apt update -q && \
    apt install -q -y libpq-dev && \
    docker-php-ext-install pdo_pgsql pgsql

RUN apt-get install -y postgresql-client rsync sendmail

RUN apt-get update && apt-get install -y \
    default-mysql-client \
    && rm -rf /var/lib/apt/lists/*

# Install Drush using Composer
RUN composer require drush/drush

CMD ["apache2-foreground"]
