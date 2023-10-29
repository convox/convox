<label @target>
	<filter **>
		@type record_transformer
		enable_ruby true
		<record>
			group_name /convox/$${record["kubernetes"]["namespace_labels"]["rack"]}/$${record["kubernetes"]["namespace_labels"]["app"]}
			stream_name service/$${record["kubernetes"]["labels"]["service"]}/$${record["kubernetes"]["pod_name"]}
			hostname ${rack}.$${record["kubernetes"]["labels"]["app"]}
			program $${record["kubernetes"]["labels"]["type"]}/$${record["kubernetes"]["labels"]["name"]}/$${record["kubernetes"]["pod_name"]}
		</record>
	</filter>

    <match rack.*.app.system.service.ingress-nginx>
      @type rewrite_tag_filter
      <rule>
        key log
        pattern  /^\{/
        tag $${tag}.access
      </rule>
    </match>

    <match rack.*.app.system.service.ingress-nginx.access>
	  @type copy

      <filter **>
        @type grep
        <regexp>
          key message
          pattern /^\{/
        </regexp>
      </filter>

      <store>
        @type cloudwatch_logs
        region ${region}
        auto_create_stream true
        retention_in_days ${access_log_retention}
        log_group_name_key group_name
        log_stream_name "/nginx-access-logs"
        message_keys log

        <buffer>
          flush_interval 1
          chunk_limit_size 2m
          queued_chunks_limit_size 32
          retry_forever true
        </buffer>
      </store>
    </match>

	<match **>
		@type copy

		<store>
			@type cloudwatch_logs
			region ${region}
			auto_create_stream true
			retention_in_days 7
			log_group_name_key group_name
			log_stream_name_key stream_name
			message_keys log
			remove_log_group_name_key true
			remove_log_stream_name_key true

			<buffer>
				flush_interval 1
				chunk_limit_size 2m
				queued_chunks_limit_size 32
				retry_forever true
			</buffer>
		</store>

		%{ for endpoint in syslog ~}
			<store>
				@type syslog
				url ${endpoint}
				facility user
				severity info
				hostname_key hostname
				tag_key program
				payload_key log
			</store>
		%{ endfor ~}
	</match>
</label>