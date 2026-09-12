package aws

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
)

var sequenceTokens sync.Map

var (
	logRetrySleep              = 3 * time.Second
	resourceNotFoundMaxRetries = 20
	logRetentionInterval       = time.Hour
	logRetentionPutInterval    = 250 * time.Millisecond
)

func (p *Provider) Log(name, stream string, ts time.Time, message string) error {
	if p.CloudwatchDisable {
		return nil
	}

	if p.AppCloudwatchDisable && name != "system" {
		return nil
	}

	group := p.appLogGroup(name)

	req := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
		LogEvents: []*cloudwatchlogs.InputLogEvent{
			{
				Timestamp: aws.Int64(ts.UnixNano() / int64(time.Millisecond)),
				Message:   aws.String(message),
			},
		},
	}

	key := fmt.Sprintf("%s/%s", *req.LogGroupName, *req.LogStreamName)

	if tv, ok := sequenceTokens.Load(key); ok {
		if token, ok := tv.(string); ok {
			req.SequenceToken = aws.String(token)
		}
	}

	for {
		res, err := p.CloudWatchLogs.PutLogEvents(req)
		switch awsErrorCode(err) {
		case "ResourceNotFoundException":
			if strings.Contains(err.Error(), "log group") {
				if err := p.createLogGroup(name); err != nil {
					return err
				}
			}
			if err := p.createLogStream(group, stream); err != nil {
				return err
			}
		case "InvalidSequenceTokenException":
			token, err := p.nextSequenceToken(group, stream)
			if err != nil {
				return err
			}
			req.SequenceToken = aws.String(token)
		case "":
			sequenceTokens.Store(key, *res.NextSequenceToken)
			return nil
		default:
			return err
		}

		continue
	}
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	if p.CloudwatchDisable || p.AppCloudwatchDisable {
		return io.NopCloser(strings.NewReader("")), nil
	}
	if p.ContextTID() != "" {
		name = fmt.Sprintf("%s-%s", p.ContextTID(), name)
	}
	return p.subscribeLogs(p.Context(), p.appLogGroup(name), "", opts)
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	if p.CloudwatchDisable {
		return io.NopCloser(strings.NewReader("")), nil
	}
	return p.subscribeLogs(p.Context(), p.appLogGroup("system"), "", opts)
}

func (p *Provider) appLogGroup(app string) string {
	return fmt.Sprintf("/convox/%s/%s", p.Name, app)
}

func (p *Provider) createLogGroup(app string) error {
	_, err := p.CloudWatchLogs.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(p.appLogGroup(app)),
		Tags: map[string]*string{
			"system": aws.String("convox"),
			"rack":   aws.String(p.Name),
			"app":    aws.String(app),
		},
	})
	if err != nil {
		return err
	}

	retention := int64(7)

	if p.CloudwatchRetentionInDays > 0 {
		retention = int64(p.CloudwatchRetentionInDays)
	}

	_, err = p.CloudWatchLogs.PutRetentionPolicy(&cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(p.appLogGroup(app)),
		RetentionInDays: aws.Int64(retention),
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) UpdateOrDisableLogGroupRetention(app string, retentionDays int, disableRetention bool) error {
	if p.CloudwatchDisable || p.AppCloudwatchDisable {
		return nil
	}

	// disable retention policy
	if disableRetention {
		_, err := p.CloudWatchLogs.DeleteRetentionPolicy(&cloudwatchlogs.DeleteRetentionPolicyInput{
			LogGroupName: aws.String(p.appLogGroup(app)),
		})
		if err != nil {
			return err
		}
		return nil
	}

	// Round retentionDays to the nearest higher allowed value
	retentionDays = roundUpToNearestAllowedRetention(retentionDays)
	_, err := p.CloudWatchLogs.PutRetentionPolicy(&cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(p.appLogGroup(app)),
		RetentionInDays: aws.Int64(int64(retentionDays)),
	})
	if err != nil {
		return err
	}

	return nil
}

func roundUpToNearestAllowedRetention(retentionDays int) int {
	allowedRetentionValues := []int{1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653}

	// Find the first allowed retention value that is greater than or equal to retentionDays
	for _, allowedValue := range allowedRetentionValues {
		if allowedValue >= retentionDays {
			return allowedValue
		}
	}

	return allowedRetentionValues[len(allowedRetentionValues)-1]
}

// The third return is false for an awsLogs block carrying neither a positive
// cwRetention nor disableRetention, which expresses no preference.
func appLogRetention(m *manifest.Manifest) (int, bool, bool) {
	l := m.AppSettings.AwsLogs

	switch {
	case l == nil:
		return 0, false, false
	case l.RetentionDisable:
		return 0, true, true
	case l.CwRetention > 0:
		return l.CwRetention, false, true
	default:
		return 0, false, false
	}
}

func normalizeRetention(v string) int {
	n, _ := strconv.Atoi(v)

	if n < 1 {
		return 0
	}

	return roundUpToNearestAllowedRetention(n)
}

func (p *Provider) applyAppLogRetention(app string, m *manifest.Manifest) error {
	days, disable, ok := appLogRetention(m)
	if !ok {
		return nil
	}

	return p.UpdateOrDisableLogGroupRetention(app, days, disable)
}

func (p *Provider) runLogRetentionReconciler(ctx context.Context) {
	if p.CloudwatchRetentionInDays < 1 {
		return
	}

	p.reconcileLogRetentionSafe(ctx)

	tick := time.NewTicker(logRetentionInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reconcileLogRetentionSafe(ctx)
		}
	}
}

func (p *Provider) reconcileLogRetentionSafe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=log_retention at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	if err := p.reconcileLogRetention(ctx); err != nil {
		fmt.Printf("ns=log_retention at=warn kind=reconcile rack=%s err=%q\n", p.Name, err)
	}
}

func (p *Provider) reconcileLogRetention(ctx context.Context) error {
	days := int64(p.CloudwatchRetentionInDays)

	skip, err := p.logRetentionOverrides()
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("/convox/%s/", p.Name)

	var rerr error

	err = p.CloudWatchLogs.DescribeLogGroupsPages(&cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(prefix),
	}, func(page *cloudwatchlogs.DescribeLogGroupsOutput, last bool) bool {
		for _, g := range page.LogGroups {
			if ctx.Err() != nil {
				return false
			}

			group := aws.StringValue(g.LogGroupName)

			if group == "" || skip[group] || aws.Int64Value(g.RetentionInDays) == days {
				continue
			}

			if err := p.putLogGroupRetention(group, days); err != nil {
				rerr = fmt.Errorf("%s: %s", group, err)
			}
		}

		return true
	})
	if err != nil {
		return err
	}

	if err := p.reconcileEksLogRetention(days); err != nil {
		rerr = err
	}

	return rerr
}

// An app whose manifest cannot be read joins the set, because an override that
// cannot be seen must not be overwritten.
func (p *Provider) logRetentionOverrides() (map[string]bool, error) {
	nss, err := p.ListNamespacesFromInformer(fmt.Sprintf("system=convox,rack=%s,type=app", p.Name))
	if err != nil {
		return nil, err
	}

	skip := map[string]bool{}

	for i := range nss.Items {
		ns := nss.Items[i]

		app := common.CoalesceString(ns.Labels["app"], ns.Labels["name"])
		if app == "" {
			continue
		}

		// A tenant app's namespace carries a tid that its fluentd group name does
		// not, and the event controller writes a second group that does.
		tid := ns.Labels["tid"]

		groups := []string{p.appLogGroup(app)}

		gp := structs.Provider(p)

		if tid != "" {
			groups = append(groups, p.appLogGroup(fmt.Sprintf("%s-%s", tid, app)))
			gp = p.WithContext(context.WithValue(p.Context(), structs.ConvoxTIDCtxKey, tid))
		}

		// The atom carries the active release before the annotation mirroring it
		// does, so a promote in flight must not be read as the previous release.
		release := ns.Annotations["convox.com/app-release"]

		if _, id, serr := p.Atom.Status(ns.Name, "app"); serr == nil && id != "" {
			release = id
		}

		if release == "" {
			continue
		}

		m, _, merr := common.ReleaseManifest(gp, app, release)

		keep := merr != nil
		if merr == nil {
			_, _, keep = appLogRetention(m)
		}

		if keep {
			for _, g := range groups {
				skip[g] = true
			}
		}
	}

	return skip, nil
}

func (p *Provider) reconcileEksLogRetention(days int64) error {
	group := fmt.Sprintf("/aws/eks/%s/cluster", p.Name)

	res, err := p.CloudWatchLogs.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(group),
	})
	if err != nil {
		return err
	}

	for _, g := range res.LogGroups {
		if aws.StringValue(g.LogGroupName) != group {
			continue
		}

		if aws.Int64Value(g.RetentionInDays) == days {
			return nil
		}

		return p.putLogGroupRetention(group, days)
	}

	return nil
}

func (p *Provider) putLogGroupRetention(group string, days int64) error {
	time.Sleep(logRetentionPutInterval)

	_, err := p.CloudWatchLogs.PutRetentionPolicy(&cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(group),
		RetentionInDays: aws.Int64(days),
	})
	if awsErrorCode(err) == cloudwatchlogs.ErrCodeResourceNotFoundException {
		return nil
	}

	return err
}

func (p *Provider) createLogStream(group, stream string) error {
	_, err := p.CloudWatchLogs.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) nextSequenceToken(group, stream string) (string, error) {
	res, err := p.CloudWatchLogs.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(group),
		LogStreamNamePrefix: aws.String(stream),
	})
	if err != nil {
		return "", err
	}
	if len(res.LogStreams) != 1 {
		return "", fmt.Errorf("could not describe log stream: %s/%s", group, stream)
	}
	if res.LogStreams[0].UploadSequenceToken == nil {
		return "", fmt.Errorf("could not fetch sequence token for log stream: %s/%s", group, stream)
	}

	return *res.LogStreams[0].UploadSequenceToken, nil
}

func (p *Provider) streamLogs(ctx context.Context, w io.WriteCloser, group, stream string, opts structs.LogsOptions) error {
	defer w.Close()

	req := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(group),
	}

	if opts.Filter != nil {
		req.FilterPattern = aws.String(*opts.Filter)
	}

	follow := common.DefaultBool(opts.Follow, true)

	var start int64

	if opts.Since != nil {
		start = time.Now().UTC().Add((*opts.Since)*-1).UnixNano() / int64(time.Millisecond)
		req.StartTime = aws.Int64(start)
	}

	if stream != "" {
		req.LogStreamNames = []*string{aws.String(stream)}
	} else {
		req.Interleaved = aws.Bool(true)
	}

	seen := map[string]bool{}
	rnf := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// check for closed writer
			if _, err := w.Write([]byte{}); err != nil {
				return err
			}

			res, err := p.CloudWatchLogs.FilterLogEvents(req)
			if err != nil {
				switch awsErrorCode(err) {
				case "ThrottlingException":
					time.Sleep(logRetrySleep)
					continue
				case "ResourceNotFoundException":
					// The group is created on the app's first log write, so a fresh
					// tail may briefly miss; retry bounded rather than forever (a group
					// that never appears would otherwise be a permanent FilterLogEvents storm).
					rnf++
					if rnf >= resourceNotFoundMaxRetries {
						return nil
					}
					time.Sleep(logRetrySleep)
					continue
				default:
					return err
				}
			}

			rnf = 0

			es := []*cloudwatchlogs.FilteredLogEvent{}

			for _, e := range res.Events {
				if !seen[*e.EventId] {
					es = append(es, e)
					seen[*e.EventId] = true
				}

				if e.Timestamp != nil && *e.Timestamp > start {
					start = *e.Timestamp
				}
			}

			sort.Slice(es, func(i, j int) bool { return *es[i].Timestamp < *es[j].Timestamp })

			if _, err := writeLogEvents(w, es, opts); err != nil {
				return err
			}

			req.NextToken = res.NextToken

			if res.NextToken != nil {
				time.Sleep(2 * time.Second)
			} else if len(es) == 0 {
				time.Sleep(5 * time.Second)
			} else {
				time.Sleep(1 * time.Second)
			}

			if res.NextToken == nil {
				if !follow {
					return nil
				}

				req.StartTime = aws.Int64(start)
			}
		}
	}
}

func (p *Provider) subscribeLogs(ctx context.Context, group, stream string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.streamLogs(ctx, w, group, stream, opts)

	return r, nil
}

func writeLogEvents(w io.Writer, events []*cloudwatchlogs.FilteredLogEvent, opts structs.LogsOptions) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}

	latest := int64(0)

	for _, e := range events {
		if *e.Timestamp > latest {
			latest = *e.Timestamp
		}

		prefix := ""

		if common.DefaultBool(opts.Prefix, false) {
			sec := *e.Timestamp / 1000
			nsec := (*e.Timestamp % 1000) * 1000
			t := time.Unix(sec, nsec).UTC()

			prefix = fmt.Sprintf("%s %s ", t.Format(time.RFC3339), *e.LogStreamName)
		}

		line := fmt.Sprintf("%s%s\n", prefix, strings.TrimSuffix(*e.Message, "\n"))

		if _, err := w.Write([]byte(line)); err != nil {
			return 0, err
		}
	}

	return latest, nil
}
