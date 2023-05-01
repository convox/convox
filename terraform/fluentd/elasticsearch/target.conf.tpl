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