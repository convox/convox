<label @target>
	<filter **>
		@type record_transformer
		enable_ruby true
		<record>
			index convox.$${record["kubernetes"]["namespace_labels"]["rack"]}.$${record["kubernetes"]["namespace_labels"]["app"]}
			stream service.$${record["kubernetes"]["labels"]["service"]}.$${record["kubernetes"]["pod_name"]}
		</record>
	</filter>

	<match **>
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
	</match>
</label>