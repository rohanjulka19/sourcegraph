import BellIcon from 'mdi-react/BellIcon'
import GithubCircleIcon from 'mdi-react/GithubCircleIcon'
import UserGroupIcon from 'mdi-react/UserGroupIcon'
import UserIcon from 'mdi-react/UserIcon'
import React from 'react'
import { Toggle } from '../../../../../../shared/src/components/Toggle'
import { ExtensionsControllerNotificationProps } from '../../../../../../shared/src/extensions/controller'
import * as GQL from '../../../../../../shared/src/graphql/schema'
import { isDefined } from '../../../../../../shared/src/util/types'
import { InfoSidebarSection } from '../../../../components/infoSidebar/InfoSidebar'
import { CampaignsIcon } from '../../../expCampaigns/icons'
import { ObjectCampaignsList } from '../../../expCampaigns/object/ObjectCampaignsList'
import { LabelIcon } from '../../../labels/icons'
import { LabelableLabelsDropdownButton } from '../../../labels/labelable/LabelableLabelsDropdownButton'
import { LabelableLabelsList } from '../../../labels/labelable/LabelableLabelsList'
import { ThreadStateBadge } from '../../common/threadState/ThreadStateBadge'
import { ThreadStateIcon } from '../../common/threadState/ThreadStateIcon'
import { CopyThreadLinkButton } from './CopyThreadLinkButton'
import { ThreadCampaignsDropdownButton } from './ThreadCampaignsDropdownButton'

interface Props extends ExtensionsControllerNotificationProps {
    thread: GQL.IThread
    onThreadUpdate: () => void
}

const SAMPLE_THREAD_SIDEBAR = false

export const threadSidebarSections = ({ thread, onThreadUpdate, ...props }: Props): InfoSidebarSection[] =>
    [
        {
            expanded: <ThreadStateBadge thread={thread} className="w-100" />,
            collapsed: <ThreadStateIcon thread={thread} />,
        },
        {
            expanded: {
                title: (
                    <ThreadCampaignsDropdownButton
                        {...props}
                        thread={thread}
                        onChange={onThreadUpdate}
                        buttonClassName="btn-link p-0"
                    />
                ),
                children: <ObjectCampaignsList object={thread} icon={false} itemClassName="small" />,
            },
            collapsed: {
                icon: CampaignsIcon,
                tooltip: 'Campaign',
            },
        },
        SAMPLE_THREAD_SIDEBAR
            ? {
                  expanded: {
                      title: 'Assignee',
                      children: <strong>@sqs</strong>,
                  },
                  collapsed: {
                      icon: UserIcon,
                      tooltip: 'Assignee: @sqs',
                  },
              }
            : null,
        {
            expanded: {
                title: (
                    <LabelableLabelsDropdownButton
                        {...props}
                        labelable={thread}
                        repository={thread.repository}
                        onChange={onThreadUpdate}
                        buttonClassName="btn-link p-0"
                    />
                ),
                children: <LabelableLabelsList labelable={thread} itemClassName="small mr-2" />,
            },
            collapsed: {
                icon: LabelIcon,
                tooltip: 'Labels',
            },
        },
        SAMPLE_THREAD_SIDEBAR
            ? {
                  expanded: {
                      title: '3 participants',
                      children: <div className="text-muted">@sqs @jtal3sf @xyzhao</div>,
                  },
                  collapsed: {
                      icon: UserGroupIcon,
                      tooltip: '3 participants',
                  },
              }
            : null,
        SAMPLE_THREAD_SIDEBAR
            ? {
                  expanded: {
                      title: (
                          <div className="d-flex align-items-center justify-content-between">
                              Notifications <Toggle value={true} />
                          </div>
                      ),
                  },
                  collapsed: {
                      icon: BellIcon,
                      tooltip: 'Notifications: on',
                  },
              }
            : null,
        {
            expanded: {
                title: (
                    <div className="d-flex align-items-center justify-content-between">
                        Link{' '}
                        <CopyThreadLinkButton
                            link={thread.url}
                            className="btn btn-link btn-link-sm text-decoration-none px-0"
                        >
                            #{thread.number}
                        </CopyThreadLinkButton>
                    </div>
                ),
            },
            collapsed: (
                <CopyThreadLinkButton
                    link={thread.url}
                    className="btn btn-link btn-link-sm text-decoration-none px-0"
                />
            ),
        },
        ...thread.externalURLs.map(({ url, serviceType }) => ({
            expanded: {
                title: (
                    <div className="d-flex align-items-center justify-content-between position-relative">
                        {serviceType === 'github' ? 'GitHub' : serviceType}
                        <a href={url} target="_blank" className="stretched-link" rel="noopener noreferrer">
                            <GithubCircleIcon className="icon-inline" />
                        </a>
                    </div>
                ),
            },
            collapsed: (
                <a href={url} target="_blank" rel="noopener noreferrer">
                    <GithubCircleIcon className="icon-inline" />
                </a>
            ),
        })),
    ].filter(isDefined)