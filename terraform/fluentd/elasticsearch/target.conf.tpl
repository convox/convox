<label @target>
	<filter **>
		@type record_transformer
		enable_ruby true
		<record>
			index convox.$${record["kubernetes"]["namespace_labels"]["rack"]}.$${time.getutc.strftime("%Y.%m.%d")}
			app $${record["kubernetes"]["namespace_labels"]["app"]}
			stream $${record["kubernetes"]["labels"]["type"]}.$${record["kubernetes"]["labels"]["name"]}.$${record["kubernetes"]["pod_name"]}
			hostname ${rack}.$${record["kubernetes"]["labels"]["app"]}
			program $${record["kubernetes"]["labels"]["type"]}/$${record["kubernetes"]["labels"]["name"]}/$${record["kubernetes"]["pod_name"]}
			log $${record["log"]+"\n"}
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

	<match **>
		@type copy

		<store>
			@type elasticsearch
			host ${elasticsearch}
			port 9200
			reload_connections false
			reconnect_on_error true
			target_index_key index
			type_name _doc
			logstash_format true
			unrecoverable_error_types ["out_of_memory_error", "es_rejected_execution_exception", "cluster_block_exception"]
			<buffer>
				@type file
				path /var/log/fluentd-buffers/elasticsearch
				file_permission 0600
				dir_permission 0700
				flush_interval 1
				flush_at_shutdown true
				flush_thread_burst_interval 0.1
				chunk_limit_size 2m
				total_limit_size ${buffer_size}m
				queued_chunks_limit_size 32
				overflow_action block
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
</label>