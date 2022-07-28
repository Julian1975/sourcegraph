import React from 'react'

import classNames from 'classnames'
import { parseISO } from 'date-fns'
import format from 'date-fns/format'

import * as GQL from '@sourcegraph/shared/src/schema'
import { Alert, Tooltip } from '@sourcegraph/wildcard'

import { formatRelativeExpirationDate, isProductLicenseExpired } from '../../../productSubscription/helpers'

/**
 * Displays an alert indicating the validity of a product license.
 */
export const ProductLicenseValidity: React.FunctionComponent<
    React.PropsWithChildren<{
        licenseInfo: GQL.IProductLicenseInfo
        primary: boolean
        className?: string
    }>
> = ({ licenseInfo: { expiresAt }, primary, className = '' }) => {
    const isExpired = isProductLicenseExpired(expiresAt)
    const tooltip = format(parseISO(expiresAt), 'PPpp')
    const validityClass = isExpired ? 'danger' : 'success'

    if (primary) {
        return (
            <Tooltip content={tooltip}>
                <Alert className={classNames(className, 'py-1 px-2')} variant={isExpired ? 'danger' : 'success'}>
                    <strong>{isExpired ? 'Expired' : 'Valid'}</strong> ({formatRelativeExpirationDate(expiresAt)})
                </Alert>
            </Tooltip>
        )
    }

    return (
        <Tooltip content={tooltip}>
            <div className={className}>
                <strong className={`text-${validityClass}`}>{isExpired ? 'Expired' : 'Valid'}</strong> (
                {formatRelativeExpirationDate(expiresAt)})
            </div>
        </Tooltip>
    )
}
