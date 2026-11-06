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
%{ if redact_mode != "" ~}

	<filter rack.*.app.system.service.ingress-nginx rack.*.app.system.service.ingress-nginx-internal>
		@type record_modifier
%{ if redact_mode == "all" ~}
		prepare_value @r = /([?&#"])([^=&?#"\s]+)=[^&#"\s]*/; @f = lambda { |s| (s.is_a?(String) && s.include?("=")) ? s.gsub(@r, '\1\2=FILTERED') : s }
%{ else ~}
		prepare_value @r = /([?&#"])((?:${redact_alternation})=)[^&#"\s]*/i; @f = lambda { |s| (s.is_a?(String) && s.include?("=")) ? s.gsub(@r, '\1\2FILTERED') : s }
%{ endif ~}
		<record>
			log $${@f.call(record["log"])}
		</record>
	</filter>
%{ endif ~}

    <match rack.*.app.system.service.ingress-nginx>
      @type rewrite_tag_filter
      <rule>
        key log
        pattern  /^\{/
        tag $${tag}.access
      </rule>
      <rule>
        key log
        pattern  /^\{/
        invert true
        tag $${tag}.other
      </rule>
    </match>

    <match rack.*.app.system.service.ingress-nginx.access>
      @type copy
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
          total_limit_size ${buffer_size}m
          queued_chunks_limit_size 32
          overflow_action ${app_cloudwatch_disable ? "throw_exception" : "block"}
          retry_forever true
          retry_max_interval 30s
        </buffer>
      </store>
%{ if syslog_access_logs ~}
%{ for endpoint in syslog ~}

      <store>
        @type syslog
        url ${endpoint}
        facility user
        severity info
        hostname_key hostname
        tag_key program
        payload_key log
        tls_verify ${syslog_tls_verify}
      </store>
%{ endfor ~}
%{ endif ~}
    </match>

	<match ${app_cloudwatch_disable ? "rack.*.app.system.**" : "**"}>
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
				total_limit_size ${buffer_size}m
				queued_chunks_limit_size 32
				overflow_action ${app_cloudwatch_disable ? "throw_exception" : "block"}
				retry_forever true
				retry_max_interval 30s
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
				tls_verify ${syslog_tls_verify}
			</store>
		%{ endfor ~}
	</match>
%{ if app_cloudwatch_disable ~}

	<match **>
%{ if length(syslog) > 0 ~}
		@type copy

%{ for endpoint in syslog ~}
		<store>
			@type syslog
			url ${endpoint}
			facility user
			severity info
			hostname_key hostname
			tag_key program
			payload_key log
			tls_verify ${syslog_tls_verify}
		</store>
%{ endfor ~}
%{ else ~}
		@type null
%{ endif ~}
	</match>
%{ endif ~}
</label>