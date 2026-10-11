<label @target>
	<filter **>
		@type record_transformer
		enable_ruby true
		<record>
			index convox.$${record["kubernetes"]["namespace_labels"]["rack"]}.$${record["kubernetes"]["namespace_labels"]["app"]}
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
			target_index_key index
			type_name _doc
			logstash_format true
			<buffer>
				flush_interval 1
				chunk_limit_size 2m
				total_limit_size ${buffer_size}m
				queued_chunks_limit_size 32
				overflow_action block
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
				tls_verify ${syslog_tls_verify}
			</store>
		%{ endfor ~}
	</match>
</label>